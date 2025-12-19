FROM scratch
COPY cmd/device-wiresink /device-wiresink
COPY cmd/res/ /res/

EXPOSE 59910

ENTRYPOINT ["/device-wiresink"]
CMD ["-cp=keeper.http://edgex-core-keeper:59890", "--registry"]
