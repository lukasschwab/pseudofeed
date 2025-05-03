package main

import (
	_ "embed"
	"flag"
	"slices"

	"html/template"
	"net/http"
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

var (
	file string
	tmpl *template.Template
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
		feed.Items = append(feed.Items, item)

		bytes, err := feed.ToJSON()
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

	port := flag.String("port", "3000", "Port to run the server on")
	flag.Parse()
	app.Listen(*port)
}

func toNewItem(url string, now time.Time) jsonfeed.Item {
	return jsonfeed.Item{
		ID:            url,
		ExternalURL:   url,
		DatePublished: now,
	}
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
