FROM scratch
ADD main /
# TODO compile these resources inside main binary
ADD i18n /
ADD i18n/en-us.all.json /i18n/en-us.all.json
ADD i18n/ru-ru.all.json /i18n/ru-ru.all.json

ADD ca-certificates.crt /etc/ssl/certs/
CMD ["/main"]