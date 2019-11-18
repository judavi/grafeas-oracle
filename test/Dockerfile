FROM docker.pkg.github.com/judavi/grafeas-oracle/grafeas-oracle:latest 
COPY ./config.yaml /config.yaml
COPY ./wait.sh /wait.sh
COPY ./start.sh /start.sh
EXPOSE 8080

ENTRYPOINT ["./start.sh"]