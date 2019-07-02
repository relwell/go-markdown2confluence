package markdown2confluence

import (
	"fmt"
	"strings"
	"io/ioutil"

	"github.com/relwell/go-confluence"
	"github.com/gernest/front"
)

// MarkdownFile contains information about the file to upload
type MarkdownFile struct {
	Path     string
	Title    string
	Parents  []string
	Ancestor string
}

func (f *MarkdownFile) String() (url string) {
	return fmt.Sprintf("Path: %s, Title: %s, Parent: %s, Ancestor: %s", f.Path, f.Title, f.Parents, f.Ancestor)
}

func (f *MarkdownFile) getTitle(fm map[string]interface{}) string {
	if val, ok := fm["title"]; ok {
		return fmt.Sprintf("%v", val)
	}
	return strings.Replace(f.Title, "-", " ", 0)
}

// Upload a markdown file
func (f *MarkdownFile) Upload(m *Markdown2Confluence) (url string, err error) {
	var ancestorID string
	// Content of Wiki
	dat, err := ioutil.ReadFile(f.Path)
	if err != nil {
		return url, fmt.Errorf("Could not open file %s:\n\t%s", f.Path, err)
	}

	wikiContent := string(dat)

	mat := front.NewMatter()
	mat.Handle("---", front.YAMLHandler)
	matter, body, err := mat.Parse(strings.NewReader(wikiContent))

	if err != nil {
		return url, fmt.Errorf("Could not parse front matter %s:\n\t%s", f.Path, err)
	}

	wikiContent = renderContent(body)

	wikiContent += fmt.Sprintf("<br /><p style=\"color: #ccc\"><i>Article imported from <a href=\"%s\">UT Internal Documentation</a>.","https://github.com/usertesting/ut_internal_documentation")

	if created_date, ok := matter["date"]; ok {
		 wikiContent += fmt.Sprintf("Original date of creation: %s.</i>", created_date)
	}

	wikiContent += "</i></p>"

	initialLabel := &confluence.Label{Name: "migrated-from-hugo"}
	labels := []confluence.Label{*initialLabel}

	if tags, ok := matter["tags"]; ok {
		if tagList, ok := tags.([]interface{}); ok {
			for _, tag := range tagList {
				newLabel := &confluence.Label{Name: fmt.Sprintf("%v", tag)}
				labels = append(labels[:], *newLabel)
				fmt.Sprintf("%v", labels)
			}
		}
	}

	if m.Debug {
		fmt.Println("---- RENDERED CONTENT START ---------------------------------")
		fmt.Println(f.getTitle(matter))
		fmt.Println("%v", matter)
		fmt.Println("%v", labels)
		// fmt.Println(wikiContent)
		fmt.Println("---- RENDERED CONTENT END -----------------------------------")
	}

	// Create the Confluence client
	client := new(confluence.Client)
	client.Username = m.Username
	client.Password = m.Password
	client.Endpoint = m.Endpoint
	client.Debug = m.Debug

	// search for existing page
	contentResults, err := client.GetContent(&confluence.GetContentQueryParameters{
		Title:    f.getTitle(matter),
		Spacekey: m.Space,
		Limit:    1,
		Type:     "page",
		Expand:   []string{"version", "body.storage"},
	})
	if err != nil {
		return url, fmt.Errorf("Error checking for existing page: %s", err)
	}

	if len(f.Parents) > 0 {
		ancestorID, err = f.FindOrCreateAncestors(m, client)
		if err != nil {
			return url, err
		}
	}

	// if page exists, update it
	if len(contentResults) > 0 {
		content := contentResults[0]
		content.Version.Number++
		content.Body.Storage.Representation = "storage"
		content.Body.Storage.Value = wikiContent

		if ancestorID != "" {
			content.Ancestors = append(content.Ancestors, Ancestor{
				ID: ancestorID,
			})
		}

		content, err = client.UpdateContent(&content, nil)
		if err != nil {
			return url, fmt.Errorf("Error updating content: %s", err)
		}
		url = client.Endpoint + content.Links.Tinyui

		// if page does not exist, create it
	} else {

		bp := confluence.CreateContentBodyParameters{}
		bp.Title = f.Title
		bp.Type = "page"
		bp.Space.Key = m.Space
		bp.Body.Storage.Representation = "storage"
		bp.Body.Storage.Value = wikiContent

		if ancestorID != "" {
			bp.Ancestors = append(bp.Ancestors, Ancestor{
				ID: ancestorID,
			})
		}

		content, err := client.CreateContent(&bp, nil)
		if err != nil {
			return url, fmt.Errorf("Error creating page: %s", err)
		}
	// update labels
	content.Metadata = &confluence.Metadata{
		Labels: labels,
	}
	content.Version.Number++
	content, err = client.UpdateContent(&content, nil)

		url = client.Endpoint + content.Links.Tinyui
	}

	return url, nil
}

// FindOrCreateAncestors creates an empty page to represent a local "folder" name
func (f *MarkdownFile) FindOrCreateAncestors(m *Markdown2Confluence, client *confluence.Client) (ancestorID string, err error) {

	for _, parent := range f.Parents {
		ancestorID, err = f.FindOrCreateAncestor(m, client, ancestorID, parent)
		if err != nil {
			return
		}
	}
	return
}

// ParentIndex caches parent page Ids for futures reference
var ParentIndex = make(map[string]string)

// FindOrCreateAncestor creates an empty page to represent a local "folder" name
func (f *MarkdownFile) FindOrCreateAncestor(m *Markdown2Confluence, client *confluence.Client, ancestorID, parent string) (string, error) {
	if parent == "" {
		return "", nil
	}

	if val, ok := ParentIndex[parent]; ok {
		return val, nil
	}

	if m.Debug {
		fmt.Printf("Searching for parent %s\n", parent)
	}

	contentResults, err := client.GetContent(&confluence.GetContentQueryParameters{
		Title:    parent,
		Spacekey: m.Space,
		Limit:    1,
		Type:     "page",
	})
	if err != nil {
		return "", fmt.Errorf("Error checking for parent page: %s", err)
	}

	if len(contentResults) > 0 {
		content := contentResults[0]
		ParentIndex[parent] = content.ID
		return content.ID, err
	}

	// if parent page does not exist, create it
	bp := confluence.CreateContentBodyParameters{}
	bp.Title = parent
	bp.Type = "page"
	bp.Space.Key = m.Space
	bp.Body.Storage.Representation = "storage"
	bp.Body.Storage.Value = defaultAncestorPage

	if m.Debug {
		fmt.Printf("Creating parent page '%s' with ancestor id %s\n", bp.Title, ancestorID)
	}

	if ancestorID != "" {
		bp.Ancestors = append(bp.Ancestors, Ancestor{
			ID: ancestorID,
		})
	}

	content, err := client.CreateContent(&bp, nil)
	if err != nil {
		return "", fmt.Errorf("Error creating parent page %s for %s: %s", f.Path, bp.Title, err)
	}
	ParentIndex[parent] = content.ID
	return content.ID, nil
}

// Ancestor TODO: move this to go-confluence api
type Ancestor struct {
	ID string `json:"id,omitempty"`
}

const defaultAncestorPage = `
<p>
   <ac:structured-macro ac:name="children" ac:schema-version="2" ac:macro-id="a93cdc19-61cd-4c21-8da7-0af3c6b76c07">
      <ac:parameter ac:name="all">true</ac:parameter>
      <ac:parameter ac:name="sort">title</ac:parameter>
   </ac:structured-macro>
</p>
`
