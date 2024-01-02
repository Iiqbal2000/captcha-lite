build:
	rm -rf out
	mkdir out

	GOOS=darwin GOARCH=amd64 go build -o captcha-lite .
	tar -czf out/captcha-lite-darwin-amd64.tar.gz captcha-lite LICENSE README.md
	rm captcha-lite
	sha256sum out/captcha-lite-darwin-amd64.tar.gz > out/captcha-lite-darwin-amd64.tar.gz.sha256sum

	GOOS=darwin GOARCH=arm64 go build -o captcha-lite .
	tar -czf out/captcha-lite-darwin-arm64.tar.gz captcha-lite LICENSE README.md
	rm captcha-lite
	sha256sum out/captcha-lite-darwin-arm64.tar.gz > out/captcha-lite-darwin-arm64.tar.gz.sha256sum

	GOOS=linux GOARCH=386 go build -o captcha-lite .
	tar -czf out/captcha-lite-linux-386.tar.gz captcha-lite LICENSE README.md
	rm captcha-lite
	sha256sum out/captcha-lite-linux-386.tar.gz > out/captcha-lite-linux-386.tar.gz.sha256sum

	GOOS=linux GOARCH=amd64 go build -o captcha-lite .
	tar -czf out/captcha-lite-linux-amd64.tar.gz captcha-lite LICENSE README.md
	rm captcha-lite
	sha256sum out/captcha-lite-linux-amd64.tar.gz > out/captcha-lite-linux-amd64.tar.gz.sha256sum

	GOOS=linux GOARCH=arm go build -o captcha-lite .
	tar -czf out/captcha-lite-linux-arm.tar.gz captcha-lite LICENSE README.md
	rm captcha-lite
	sha256sum out/captcha-lite-linux-arm.tar.gz > out/captcha-lite-linux-arm.tar.gz.sha256sum

	GOOS=linux GOARCH=arm64 go build -o captcha-lite .
	tar -czf out/captcha-lite-linux-arm64.tar.gz captcha-lite LICENSE README.md
	rm captcha-lite
	sha256sum out/captcha-lite-linux-arm64.tar.gz > out/captcha-lite-linux-arm64.tar.gz.sha256sum

	GOOS=windows GOARCH=386 go build -o captcha-lite.exe .
	zip out/captcha-lite-windows-386.zip captcha-lite.exe LICENSE README.md
	rm captcha-lite.exe
	sha256sum out/captcha-lite-windows-386.zip > out/captcha-lite-windows-386.zip.sha256sum

	GOOS=windows GOARCH=amd64 go build -o captcha-lite.exe .
	zip out/captcha-lite-windows-amd64.zip captcha-lite.exe LICENSE README.md
	rm captcha-lite.exe
	sha256sum out/captcha-lite-windows-amd64.zip > out/captcha-lite-windows-amd64.zip.sha256sum

	GOOS=windows GOARCH=arm go build -o captcha-lite.exe .
	zip out/captcha-lite-windows-arm.zip captcha-lite.exe LICENSE README.md
	rm captcha-lite.exe
	sha256sum out/captcha-lite-windows-arm.zip > out/captcha-lite-windows-arm.zip.sha256sum
