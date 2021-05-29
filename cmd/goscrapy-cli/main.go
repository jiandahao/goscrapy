package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/urfave/cli"
)

// TODO:
// merge import; support appending;
var spiderImplTemplate = `package {{.PackageName}}

import (
	"github.com/jiandahao/goscrapy"
)

// {{.Name}} spider
type {{.Name}} struct{}

// Name returns the spider name
func (s *{{.Name}}) Name() string {
	return "{{.Name}}"
}

// StartRequests returns start requests. These request will be push into spider scheduler
// at initialized time
func (s *{{.Name}}) StartRequests() []*goscrapy.Request {
	return nil
}

// URLMatcher returns the url matcher
func (s *{{.Name}}) URLMatcher() goscrapy.URLMatcher {
	return goscrapy.NewStaticStringMatcher("https://www.example.com")
}

// Parse parse response
func (s *{{.Name}}) Parse(ctx *goscrapy.Context) (*goscrapy.Items, []*goscrapy.Request, error) {
	return nil, nil, nil
}
`

var projImplTemplate = `package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/jiandahao/goscrapy"
	"github.com/jiandahao/goutils/logger"
)

func main() {

	eng := goscrapy.NewEngine()

	eng.UseLogger(logger.NewSugaredLogger("engine", "debug"))

	eng.RegisterSipders(/*add your own spiders here*/)
	eng.RegisterPipelines(/*add your own pipelines here*/)

	go eng.Start()

	defer eng.Stop()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
`

// SpiderConfig spider config
type SpiderConfig struct {
	Name        string
	PackageName string
}

func main() {
	app := cli.NewApp()

	app.Name = "goscrapy-cli"
	app.Usage = "Tools for auto-generating your own goscrapy project"
	app.Version = "v1.0.0"
	app.UsageText = "goscrapy-cli [command] [command options] [arguments...]"
	app.Commands = []cli.Command{
		CreateCommand(),
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

// CreateCommand returns command using to create project / spider code
func CreateCommand() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "create goscrapy project / spiders",
		Subcommands: []cli.Command{
			CreateProjectCommand(),
			CreateSpiderCommand(),
			CreatePipelineCommand(),
		},
	}
}

// CreateProjectCommand returns command using to create goscrapy project
func CreateProjectCommand() cli.Command {
	return cli.Command{
		Name:  "project",
		Usage: "create your goscrapy project",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "n",
				Usage: "specified your project name",
			},
			cli.StringFlag{
				Name:  "o",
				Usage: "specified output directory to store new generated project",
			},
		},
		Action: func(ctx *cli.Context) error {
			projName := ctx.String("n")
			output := ctx.String("o")

			if !strings.HasSuffix(output, ".go") {
				output = fmt.Sprintf("%s/%s/main.go", strings.TrimSuffix(output, "/"), projName)
			}

			fmt.Printf("creating project [%s] at %s ", projName, output)

			fd, err := openFile(output)
			if err != nil {
				return err
			}

			defer fd.Close()

			if _, err := fd.WriteString(projImplTemplate); err != nil {
				return err
			}

			return nil
		},
	}
}

// CreateSpiderCommand returns command using to generate spider
func CreateSpiderCommand() cli.Command {
	return cli.Command{
		Name:  "spider",
		Usage: "create your goscrapy spider",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "o",
				Usage: "specified output path",
				Value: "./",
			},
			cli.StringFlag{
				Name:  "n",
				Usage: "specified spider name",
				Value: "DefaultSpider",
			},
			cli.StringFlag{
				Name:  "pkg",
				Usage: "package name",
				Value: "main",
			},
		},
		Action: func(ctx *cli.Context) error {
			output := ctx.String("o")
			spiderName := ctx.String("n")
			pkgName := ctx.String("pkg")

			if !strings.HasSuffix(output, ".go") {
				output = fmt.Sprintf("%s/%s.go", strings.TrimSuffix(output, "/"), strings.ToLower(spiderName))
			}

			fmt.Printf("creating spider [%s:%s] at %s", pkgName, spiderName, output)
			fd, err := openFile(output)
			if err != nil {
				return err
			}
			defer fd.Close()

			tmpl, err := template.New("spider").Parse(spiderImplTemplate)
			if err != nil {
				return err
			}

			return tmpl.Execute(fd, &SpiderConfig{
				Name:        spiderName,
				PackageName: pkgName,
			})
		},
	}
}

var pipelineTempl = `package {{.Package}}

import (
	"github.com/jiandahao/goscrapy"
)

// {{.Name}} a simple pipeline
type {{.Name}} struct{}

// Name returns pipeline's name, it's the identity of pipeline, make sure every
// pipeline has it's own unique name
func (p *{{.Name}}) Name() string {
	return "simple_pipeline"
}

// ItemList declares all items that this pipeline cares about.
func (p *{{.Name}}) ItemList() []string {
	return []string{ {{.ItemList}} }
}

// Handle handle items
func (p *{{.Name}}) Handle(item *goscrapy.Items) error {
	if item == nil {
		return nil
	}
	return nil
}
`

// PipelineConfig pipeline config
type PipelineConfig struct {
	Name     string
	Package  string
	ItemList string
}

// CreatePipelineCommand create pipeline command
func CreatePipelineCommand() cli.Command {
	return cli.Command{
		Name:  "pipeline",
		Usage: "create you own pipeline to handle structure data from spiders",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "o",
				Usage: "specified output path",
				Value: "./",
			},
			cli.StringFlag{
				Name:  "n",
				Usage: "specified pipeline name",
				Value: "DefaultPipeline",
			},
			cli.StringFlag{
				Name:  "pkg",
				Usage: "package name",
				Value: "main",
			},
			cli.StringSliceFlag{
				Name:  "item",
				Usage: "specified all items this pipeline cares about",
			},
		},
		Action: func(ctx *cli.Context) error {
			output := ctx.String("o")
			name := ctx.String("n")
			pkgName := ctx.String("pkg")
			itemList := ctx.StringSlice("item")

			if !strings.HasSuffix(output, ".go") {
				output = fmt.Sprintf("%s/%s.go", strings.TrimSuffix(output, "/"), strings.ToLower(name))
			}

			fmt.Printf("creating pipeline [%s:%s] at %s", pkgName, name, output)
			fd, err := openFile(output)
			if err != nil {
				return err
			}
			defer fd.Close()

			tmpl, err := template.New("pipeline").Parse(pipelineTempl)
			if err != nil {
				return err
			}

			return tmpl.Execute(fd, &PipelineConfig{
				Name:     name,
				Package:  pkgName,
				ItemList: "\"" + strings.Join(itemList, "\", \"") + "\"",
			})
		},
	}
}

func openFile(filePath string) (*os.File, error) {
	dir := filepath.Dir(filePath)
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}

	fd, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return fd, nil
}
