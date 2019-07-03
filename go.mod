module github.com/justmiles/go-markdown2confluence

go 1.12

replace github.com/justmiles/go-markdown2confluence/cmd => ./cmd

replace github.com/justmiles/go-markdown2confluence/markdown2confluence => ./markdown2confluence

require (
	github.com/gernest/front v0.0.0-20181129160812-ed80ca338b88
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/justmiles/mark v0.1.1-0.20190601173636-c076c124ac41
	github.com/relwell/go-confluence v0.0.0-20190702172928-889ae55d88a0
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/cobra v0.0.4
	github.com/yuin/goldmark v1.0.7
)
