install:
	go get -u -v github.com/podhmo/go-structjson/cmd/go-structjson

example:
	go-structjson --target ./examples/models/  | jq . -S | sed "s@`pwd`@./@g;" | tee ./examples/output/models.json
