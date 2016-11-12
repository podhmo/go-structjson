install:
	go get -v github.com/podhmo/go-structjson/cmd/go-structjson

example: example1 example2 example3

example1:
	go-structjson --target ./examples/models/  | jq . -S | sed "s@`echo $$GOPATH`@GOPATH@g;" | tee ./examples/output/models.json

example2:
	go-structjson --target ./examples/models2/  | jq . -S | sed "s@`echo $$GOPATH`@GOPATH@g;" | tee ./examples/output/models2.json

example3:
	go-structjson --target ./examples/email/  | jq . -S | sed "s@`echo $$GOPATH`@GOPATH@g;" | tee ./examples/output/email.json
