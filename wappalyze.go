package webanalyze

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// WappalyzerURL is the link to the latest technologies.json file in the Wappalyzer repo
const WappalyzerURL = "https://raw.githubusercontent.com/AliasIO/wappalyzer/b9b64a9ae173cf317d86b7fb6d8ccb892ea8898d/src/technologies.json"

// StringArray type is a wrapper for []string for use in unmarshalling the technologies.json
type StringArray []string

// App type encapsulates all the data about an App from technologies.json
type App struct {
	Cats     StringArray            `json:"cats"`
	CatNames []string               `json:"category_names"`
	Cookies  map[string]string      `json:"cookies"`
	Headers  map[string]string      `json:"headers"`
	Meta     map[string]StringArray `json:"meta"`
	HTML     StringArray            `json:"html"`
	Script   StringArray            `json:"script"`
	URL      StringArray            `json:"url"`
	Website  string                 `json:"website"`
	Implies  StringArray            `json:"implies"`

	HTMLRegex   []AppRegexp `json:"-"`
	ScriptRegex []AppRegexp `json:"-"`
	URLRegex    []AppRegexp `json:"-"`
	HeaderRegex []AppRegexp `json:"-"`
	MetaRegex   []AppRegexp `json:"-"`
	CookieRegex []AppRegexp `json:"-"`
}

// Category names defined by wappalyzer
type Category struct {
	Name string `json:"name"`
}

// AppsDefinition type encapsulates the json encoding of the whole technologies.json file
type AppsDefinition struct {
	Apps map[string]App      `json:"technologies"`
	Cats map[string]Category `json:"categories"`
}

type AppRegexp struct {
	Name    string
	Regexp  *regexp.Regexp
	Version string
}

func (app *App) FindInHeaders(headers http.Header) (matches [][]string, version string) {
	var v string

	for _, hre := range app.HeaderRegex {
		if headers.Get(hre.Name) == "" {
			continue
		}
		hk := http.CanonicalHeaderKey(hre.Name)
		for _, headerValue := range headers[hk] {
			if headerValue == "" {
				continue
			}
			if m, version := findMatches(headerValue, []AppRegexp{hre}); len(m) > 0 {
				matches = append(matches, m...)
				v = version
			}
		}
	}
	return matches, v
}

// UnmarshalJSON is a custom unmarshaler for handling bogus technologies.json types from wappalyzer
func (t *StringArray) UnmarshalJSON(data []byte) error {
	var s string
	var sa []string
	var na []int

	if err := json.Unmarshal(data, &s); err != nil {
		if err := json.Unmarshal(data, &na); err == nil {
			// not a string, so maybe []int?
			*t = make(StringArray, len(na))

			for i, number := range na {
				(*t)[i] = fmt.Sprintf("%d", number)
			}

			return nil
		} else if err := json.Unmarshal(data, &sa); err == nil {
			// not a string, so maybe []string?
			*t = sa
			return nil
		}
		fmt.Println(string(data))
		return err
	}
	*t = StringArray{s}
	return nil
}

// DownloadFile pulls the latest technologies.json file from the Wappalyzer github
func DownloadFile(from, to string) error {
	resp, err := http.Get(from)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(to)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, resp.Body)
	return err
}

// load apps from io.Reader
func (wa *WebAnalyzer) loadApps(r io.Reader) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(&wa.appDefs); err != nil {
		return err
	}

	for key, value := range wa.appDefs.Apps {

		app := wa.appDefs.Apps[key]

		app.HTMLRegex = compileRegexes(value.HTML)
		app.ScriptRegex = compileRegexes(value.Script)
		app.URLRegex = compileRegexes(value.URL)

		app.HeaderRegex = compileNamedRegexes(app.Headers)
		app.CookieRegex = compileNamedRegexes(app.Cookies)

		// handle special meta field where value can be a list
		// of strings. we join them as a simple regex here
		metaRegex := make(map[string]string)
		for k, v := range app.Meta {
			metaRegex[k] = strings.Join(v, "|")
		}
		app.MetaRegex = compileNamedRegexes(metaRegex)

		app.CatNames = make([]string, 0)

		for _, cid := range app.Cats {
			if category, ok := wa.appDefs.Cats[string(cid)]; ok && category.Name != "" {
				app.CatNames = append(app.CatNames, category.Name)
			}
		}

		wa.appDefs.Apps[key] = app

	}

	return nil
}

func compileNamedRegexes(from map[string]string) []AppRegexp {

	var list []AppRegexp

	for key, value := range from {

		h := AppRegexp{
			Name: key,
		}

		if value == "" {
			value = ".*"
		}

		// Filter out webapplyzer attributes from regular expression
		splitted := strings.Split(value, "\\;")

		r, err := regexp.Compile("(?i)" + splitted[0])
		if err != nil {
			continue
		}

		if len(splitted) > 1 && strings.HasPrefix(splitted[1], "version:") {
			h.Version = splitted[1][8:]
		}

		h.Regexp = r
		list = append(list, h)
	}

	return list
}

func compileRegexes(s StringArray) []AppRegexp {
	var list []AppRegexp

	for _, regexString := range s {

		// Split version detection
		splitted := strings.Split(regexString, "\\;")

		regex, err := regexp.Compile("(?i)" + splitted[0])
		if err != nil {
			// ignore failed compiling for now
			// log.Printf("warning: compiling regexp for failed: %v", regexString, err)
		} else {
			rv := AppRegexp{
				Regexp: regex,
			}

			if len(splitted) > 1 && strings.HasPrefix(splitted[0], "version") {
				rv.Version = splitted[1][8:]
			}

			list = append(list, rv)
		}
	}

	return list
}
