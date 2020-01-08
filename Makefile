build: generator

secrets.go: secrets.go.template
	cp secrets.go.template secrets.go

generator: *.go secrets.go
	go build -o generator *.go
