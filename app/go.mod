module oficinaqr

go 1.24

replace golang.org/x/image => github.com/golang/image v0.30.0

replace golang.org/x/text => github.com/golang/text v0.28.0

require (
	github.com/lib/pq v1.12.3
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	golang.org/x/image v0.0.0-00010101000000-000000000000
)

require golang.org/x/text v0.28.0
