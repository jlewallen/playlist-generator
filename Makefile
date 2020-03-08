build: generator api

secrets.go: secrets.go.template
	cp secrets.go.template secrets.go

generator: generator.go caching.go spotify.go summary.go tokens.go secrets.go server.go
	go build -o generator $^

api: api.go
	go build -o api api.go summary.go

clean:
	rm -f generator api
