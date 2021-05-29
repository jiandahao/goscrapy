## goscrapy-cli
goscrapy-cli is easy to use as a command line tool, which is helpful to create goscrapy project and spiders for your own project.

### create a new project
```bash
goscrapy-cli create project -n newproj -o ./output_dir
```

### create a new spider
```bash
goscrapy-cli create spider -n MyFirstSpider -o ./spiders -pkg spiders
```
after this, a new spider named MyFirstSpider will be created at ./spiders/myfirstspider.go

### create a new pipeline
```bash
goscrapy-cli create pipeline -n demopipeline -o ./pipelines -pkg pipelines --item itemA --item itemB
```