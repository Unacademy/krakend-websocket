module github.com/unacademy/krakend-websocket

go 1.17

require (
	github.com/gin-gonic/gin v1.7.7
	github.com/luraproject/lura v1.4.1
	nhooyr.io/websocket v1.8.6
)

require (
	github.com/devopsfaith/flatmap v0.0.0-20200601181759-8521186182fc // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.20.0 // indirect
	github.com/golang/protobuf v1.5.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.10.3 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/valyala/fastrand v1.0.0 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/luraproject/lura v1.4.1 => github.com/Unacademy/krakend v1.4.1

// Force Go 1.17 compatible versions
replace golang.org/x/net => golang.org/x/net v0.7.0

replace golang.org/x/crypto => golang.org/x/crypto v0.6.0

replace golang.org/x/sys => golang.org/x/sys v0.5.0

replace golang.org/x/text => golang.org/x/text v0.7.0

replace google.golang.org/protobuf => google.golang.org/protobuf v1.28.1
