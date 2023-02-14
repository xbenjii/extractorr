FROM alpine:3.14

COPY bin/extractorr /usr/local/bin/extractorr

ENTRYPOINT [ "/usr/local/bin/extractorr" ]