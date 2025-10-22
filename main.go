package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"slices"
	"strings"

	"html/template"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/lukasschwab/go-jsonfeed"
	opt "github.com/lukasschwab/optional"
)

const (
	ApplicationName = "pseudofeed"
)

//go:embed templates/html.tmpl
var htmlTemplate string

//go:embed templates/bookmarklet.tmpl
var bookmarkletTemplate string

var (
	file            string
	tmpl            *template.Template
	tmplBookmarklet *template.Template
)

func init() {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	file = filepath.Join(userConfigDir, ApplicationName)

	// Initialize the feed file if it doesn't exist.
	if _, err := os.Stat(file); os.IsNotExist(err) {
		log.Info("Creating new feed file: " + file)
		feed := jsonfeed.NewFeed("Pseudofeed Pages", []jsonfeed.Item{})
		bytes, err := feed.ToJSON()
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(file, bytes, 0644); err != nil {
			panic(err)
		}
	}

	tmpl, err = template.New("html").Parse(htmlTemplate)
	if err != nil {
		panic(err)
	}

	tmplBookmarklet, err = template.New("bookmarklet").Parse(bookmarkletTemplate)
	if err != nil {
		panic(err)
	}
}

func main() {
	log.Info("Using feed file: " + file)

	app := fiber.New()
	app.Use(logger.New())

	app.Get("/feed.json", func(c *fiber.Ctx) error {
		raw, err := os.ReadFile(file)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error reading file",
				"raw":   err.Error(),
			})
		}
		return c.Send(raw)
	})

	app.Get("/bookmarklet", func(c *fiber.Ctx) error {
		c.Response().Header.Set("Content-Type", "application/javascript")
		host := c.Request().URI().Host()
		type TemplateData struct {
			BaseURL string
		}
		if err := tmplBookmarklet.Execute(c, TemplateData{BaseURL: string(host)}); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error executing template",
				"raw":   err.Error(),
			})
		}

		return nil
	})

	app.Get("/", func(c *fiber.Ctx) error {
		raw, err := os.ReadFile(file)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error reading file",
				"raw":   err.Error(),
			})
		}

		feed, err := jsonfeed.Parse(raw)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error parsing JSON",
				"raw":   err.Error(),
			})
		}

		slices.Reverse(feed.Items)
		// Make the dates human-readable.
		for i := range feed.Items {
			feed.Items[i].DatePublished = parseDate(feed.Items[i].DatePublished)
		}

		c.Response().Header.Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(c, feed.Items); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error executing template",
				"raw":   err.Error(),
			})
		}

		return nil
	})

	app.Post("/", func(c *fiber.Ctx) error {
		type Request struct {
			URL string `json:"url"`
		}

		req := new(Request)
		if err := c.BodyParser(req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request",
				"raw":   err.Error(),
			})
		}
		if req.URL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "URL is required",
			})
		}

		item := toNewItem(req.URL, time.Now())

		raw, err := os.ReadFile(file)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error reading file",
				"raw":   err.Error(),
			})
		}

		feed, err := jsonfeed.Parse(raw)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error parsing stored feed",
				"raw":   err.Error(),
			})
		}

		feed.Items = append(feed.Items, item)
		if err := feed.Validate(); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Generated invalid feed",
				"raw":   err.Error(),
			})
		}

		bytes, err := json.MarshalIndent(feed, "", "\t")
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error marshaling JSON",
				"raw":   err.Error(),
			})
		}

		if err := os.WriteFile(file, bytes, 0644); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error writing file",
				"raw":   err.Error(),
			})
		}

		return nil
	})

	port := flag.String("port", "8081", "Port to run the server on")

	flag.Parse()
	if port == nil || *port == "" {
		panic("Port is required")
	}
	if !strings.HasPrefix(*port, ":") {
		*port = ":" + *port
	}

	if err := app.Listen(*port); err != nil {
		panic(err)
	}
}

func toNewItem(data string, now time.Time) jsonfeed.Item {
	title, url, ok := parseSharedFromAndroid(data)
	if !ok {
		// We can't extract a URL, so just treat the whole data as both title
		// and URL. May result in broken links, but that's better than dropping
		// the request.
		title, url = data, data
	}
	return jsonfeed.Item{
		ID:            url,
		Title:         title,
		URL:           url,
		ExternalURL:   url,
		DatePublished: now,
	}
}

// parseSharedFromAndroid compensates for Android's default share-from-chrome
// behavior. Sharing a Wikipedia page to pseudofeed via [HTTP Shortcuts] yields
// a "data" string like this rather than a raw URL:
//
//	Enver Hoxha - Wikipedia https://en.m.wikipedia.org/wiki/Enver_Hoxha
//
// This function separates a title (`Enver Hoxha - Wikipedia`) and a URL from
// that data. I'm hoping for a consistent format, where the token after the last
// space is always the URL.
//
// [HTTP Shortcuts]: https://http-shortcuts.rmy.ch/
func parseSharedFromAndroid(data string) (title, url string, ok bool) {
	data = strings.TrimSpace(data)
	lastSpace := strings.LastIndex(data, " ")
	if lastSpace == -1 {
		return "", data, false
	}
	title, url = data[:lastSpace], data[lastSpace+1:]

	// Sanity-check the URL format.
	if parsed, err := neturl.Parse(url); err != nil {
		log.Infof("Failed parsing URL: %v", err)
		return "", data, false
	} else if parsed.Host == "" {
		log.Info("Failed parsing URL: no host")
		return "", data, false
	}

	return title, url, true
}

// parseDate is unsafe.
func parseDate(datestring opt.String) opt.String {
	s := opt.ToString(datestring)
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}

	return (opt.String)(t.Format(time.DateTime))
}
