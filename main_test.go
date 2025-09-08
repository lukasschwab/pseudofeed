package main

import (
	"testing"

	"github.com/peterldowns/testy/assert"
)

func TestParseSharedFromAndroid(t *testing.T) {
	t.Run("standard wikipedia", func(t *testing.T) {
		title, url, ok := parseSharedFromAndroid("The Life and Opinions of Tristram Shandy, Gentleman - Wikipedia https://en.m.wikipedia.org/wiki/The_Life_and_Opinions_of_Tristram_Shandy,_Gentleman")
		assert.True(t, ok)
		assert.Equal(t, "The Life and Opinions of Tristram Shandy, Gentleman - Wikipedia", title)
		assert.Equal(t, "https://en.m.wikipedia.org/wiki/The_Life_and_Opinions_of_Tristram_Shandy,_Gentleman", url)
	})

	t.Run("weird", func(t *testing.T) {
		title, url, ok := parseSharedFromAndroid("On Seeing A Piece Of Our Artillery Brought Into Action by Wilfred Owen - Famous poems, famous poets. - All Poetry https://allpoetry.com/on-seeing-a-piece-of-our-artillery-brought-into-action")
		assert.True(t, ok)
		assert.Equal(t, "On Seeing A Piece Of Our Artillery Brought Into Action by Wilfred Owen - Famous poems, famous poets. - All Poetry", title)
		assert.Equal(t, "https://allpoetry.com/on-seeing-a-piece-of-our-artillery-brought-into-action", url)
	})

	t.Run("malformed URL", func(t *testing.T) {
		data := "abc def"
		_, _, ok := parseSharedFromAndroid(data)
		assert.False(t, ok)
	})

	t.Run("URL only", func(t *testing.T) {
		data := "https://en.m.wikipedia.org/wiki/The_Life_and_Opinions_of_Tristram_Shandy,_Gentleman"
		_, _, ok := parseSharedFromAndroid(data)
		assert.False(t, ok)
	})

	t.Run("URL only, no schee", func(t *testing.T) {
		data := "en.m.wikipedia.org/wiki/The_Life_and_Opinions_of_Tristram_Shandy,_Gentleman"
		_, _, ok := parseSharedFromAndroid(data)
		assert.False(t, ok)
	})
}
