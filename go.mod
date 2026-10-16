module github.com/luo-studio/go-tex

go 1.25.0

require (
	codeberg.org/go-pdf/fpdf v0.11.1
	github.com/dlclark/regexp2 v1.12.0
	golang.org/x/image v0.38.0
)

require golang.org/x/text v0.35.0 // indirect

replace codeberg.org/go-pdf/fpdf => github.com/luo-studio/fpdf v0.0.0-20260331214128-5b17f92a99a5
