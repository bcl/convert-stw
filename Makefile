build:
	go build -o ./convert-stw ./cmd/convert-stw

test:
	./convert-stw --input ./tests/bureau.doc --output ./tests/bureau.txt.test
	diff ./tests/bureau.txt.ok ./tests/bureau.txt.test
