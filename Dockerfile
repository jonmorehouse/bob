FROM golang:latest

RUN go get golang.org/x/crypto/openpgp && \
	go get gopkg.in/yaml.v2 && \
	go get github.com/jonmorehouse/safe

COPY build /build
COPY ./src /src

RUN mkdir /output && \
	cd /src && \
	GOOS=linux GOARCH=amd64 go build -o /output/bob .

# artifactor is a small command line utility for building artifacts. Download
# the latest version of this binary and include it as a local dependency
FROM jonmorehouse/downloader:latest
ARG artifactor_version=2b03ac4
RUN mkdir /output && \
	echo "downloading version=${artifactor_version}..." && \
	/bin/downloader "https://artifacts.jm.house/artifactor/${artifactor_version}" \
	/output artifactor_linux_amd64

FROM alpine:latest
COPY --from=0 /output/bob /bin/bob
COPY --from=1 /output/artifactor_linux_amd64 /bin/artifactor
ENTRYPOINT /bob
