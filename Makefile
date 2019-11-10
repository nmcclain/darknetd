
pi:
	GOOS=linux GOARCH=arm GOARM=7 go build

pizero:
	GOOS=linux GOARCH=arm GOARM=6 go build
	
osx:
	GOOS=darwin GOARCH=amd64 go build

linux:
	GOOS=linux GOARCH=amd64 go build

