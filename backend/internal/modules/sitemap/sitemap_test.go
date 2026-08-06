package sitemap

import (
	"context"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/pkg/adapters/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore is a minimal storage.Store that records the last Put (used by
// Rebuild). Only the methods Rebuild touches are implemented.
type fakeStore struct {
	putKey  string
	putBody []byte
}

func (f *fakeStore) Mode() string { return "local" }
func (f *fakeStore) PresignUpload(context.Context, string, string, int64) (string, map[string]string, error) {
	return "", nil, nil
}
func (f *fakeStore) PublicURL(string) string { return "" }
func (f *fakeStore) Put(_ context.Context, key string, r io.Reader, _ string) error {
	buf, _ := io.ReadAll(r)
	f.putKey = key
	f.putBody = buf
	return nil
}
func (f *fakeStore) Delete(context.Context, string) error     { return nil }
func (f *fakeStore) Key(storage.Kind, string) (string, error) { return "", nil }

func newBuilderWithLister(siteBase string, rows []URLRow, listErr error) *Builder {
	b := &Builder{siteBaseURL: siteBase, store: &fakeStore{}}
	b.lister = func(context.Context) ([]URLRow, error) {
		if listErr != nil {
			return nil, listErr
		}
		return rows, nil
	}
	return b
}

// mustParseXML parses the sitemap bytes; fails the test if malformed.
func mustParseXML(t *testing.T, b []byte) urlSet {
	t.Helper()
	var set urlSet
	dec := xml.NewDecoder(strings.NewReader(string(b)))
	dec.Strict = false // tolerate the namespace prefix on <link>
	require.NoError(t, dec.Decode(&set), "malformed sitemap XML: %s", string(b))
	return set
}

// urlSet mirrors the xml structure in BuildXML for test parsing. The
// namespace is the sitemap 0.9 schema; the xhtml <link> is an alternate.
type altLink struct {
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}
type urlEntry struct {
	Loc        string    `xml:"loc"`
	LastMod    string    `xml:"lastmod,omitempty"`
	Alternates []altLink `xml:"http://www.w3.org/1999/xhtml link"`
}
type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	URLs    []urlEntry `xml:"url"`
}

func TestBuildXML_EmptySiteBaseURL_Errors(t *testing.T) {
	b := newBuilderWithLister("", nil, nil)
	_, err := b.BuildXML(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SITE_BASE_URL")
}

func TestBuildXML_RelativeBase_Errors(t *testing.T) {
	b := newBuilderWithLister("example.com", nil, nil)
	_, err := b.BuildXML(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute origin")
}

func TestBuildXML_TwoLocaleEntity_HasAlternatesAndAbsLoc(t *testing.T) {
	lastMod := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	rows := []URLRow{
		{Entity: entityProduct, EntityID: 1, Locale: "en-US", Slug: "blue-vase", LastMod: lastMod},
		{Entity: entityProduct, EntityID: 1, Locale: "zh-CN", Slug: "lan-hua-ping", LastMod: lastMod},
	}
	b := newBuilderWithLister("https://jingdezhen.example.com", rows, nil)
	xmlBytes, err := b.BuildXML(context.Background())
	require.NoError(t, err)

	set := mustParseXML(t, xmlBytes)
	require.Len(t, set.URLs, 2, "one <url> per (entity, locale, slug)")

	// Each URL must carry 2 hreflang alternates (en-US + zh-CN) and an absolute loc.
	for _, u := range set.URLs {
		assert.True(t, strings.HasPrefix(u.Loc, "https://jingdezhen.example.com/"), "loc absolute: %s", u.Loc)
		assert.NotEmpty(t, u.LastMod)
		assert.Len(t, u.Alternates, 2, "2 published locales → 2 alternates")
	}

	// Assert the en-US URL points at the en-US slug + segment; zh-CN likewise.
	var enURL, zhURL *urlEntry
	for i := range set.URLs {
		if strings.Contains(set.URLs[i].Loc, "/en-us/products/blue-vase") {
			enURL = &set.URLs[i]
		}
		if strings.Contains(set.URLs[i].Loc, "/zh-cn/products/lan-hua-ping") {
			zhURL = &set.URLs[i]
		}
	}
	require.NotNil(t, enURL, "en-US URL present")
	require.NotNil(t, zhURL, "zh-CN URL present")

	// Alternates on the en-US URL must cover both locales with correct hrefs.
	hrefByLang := map[string]string{}
	for _, a := range enURL.Alternates {
		hrefByLang[a.Hreflang] = a.Href
	}
	assert.Contains(t, hrefByLang, "en-US")
	assert.Contains(t, hrefByLang, "zh-CN")
	assert.Contains(t, hrefByLang["en-US"], "/en-us/products/blue-vase")
	assert.Contains(t, hrefByLang["zh-CN"], "/zh-cn/products/lan-hua-ping")
}

func TestBuildXML_SingleLocaleEntity_StillEmitsSelfAlternate(t *testing.T) {
	lastMod := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	rows := []URLRow{
		{Entity: entityArtist, EntityID: 7, Locale: "en-US", Slug: "wang-ming", LastMod: lastMod},
	}
	b := newBuilderWithLister("https://j.example.com", rows, nil)
	xmlBytes, err := b.BuildXML(context.Background())
	require.NoError(t, err)

	set := mustParseXML(t, xmlBytes)
	require.Len(t, set.URLs, 1)
	assert.Contains(t, set.URLs[0].Loc, "/en-us/artists/wang-ming")
	assert.Len(t, set.URLs[0].Alternates, 1, "self-referencing alternate for a single-locale entity")
	assert.Equal(t, "en-US", set.URLs[0].Alternates[0].Hreflang)
}

func TestBuildXML_FourEntities_AllSegments(t *testing.T) {
	lastMod := time.Now().UTC()
	rows := []URLRow{
		{Entity: entityProduct, EntityID: 1, Locale: "en-US", Slug: "a", LastMod: lastMod},
		{Entity: entityCeramicStory, EntityID: 2, Locale: "en-US", Slug: "b", LastMod: lastMod},
		{Entity: entityActivity, EntityID: 3, Locale: "en-US", Slug: "c", LastMod: lastMod},
		{Entity: entityArtist, EntityID: 4, Locale: "en-US", Slug: "d", LastMod: lastMod},
	}
	b := newBuilderWithLister("https://j.example.com", rows, nil)
	xmlBytes, err := b.BuildXML(context.Background())
	require.NoError(t, err)

	set := mustParseXML(t, xmlBytes)
	require.Len(t, set.URLs, 4)
	locs := []string{set.URLs[0].Loc, set.URLs[1].Loc, set.URLs[2].Loc, set.URLs[3].Loc}
	joined := strings.Join(locs, "\n")
	assert.Contains(t, joined, "/products/a")
	assert.Contains(t, joined, "/ceramicstory/b")
	assert.Contains(t, joined, "/engage/c")
	assert.Contains(t, joined, "/artists/d")
}

func TestRebuild_WritesStore(t *testing.T) {
	rows := []URLRow{
		{Entity: entityProduct, EntityID: 1, Locale: "en-US", Slug: "x", LastMod: time.Now()},
	}
	b := newBuilderWithLister("https://j.example.com", rows, nil)
	fs := b.store.(*fakeStore)
	require.NoError(t, b.Rebuild(context.Background()))
	assert.Equal(t, "sitemap.xml", fs.putKey)
	assert.NotEmpty(t, fs.putBody, "store.Put received the XML bytes")
	assert.True(t, strings.HasPrefix(string(fs.putBody), "<?xml"), "starts with XML header")
}
