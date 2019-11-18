FROM golang:1.13.4
COPY . /go/src/github.com/judavi/grafeas-oracle/
WORKDIR /go/src/github.com/judavi/grafeas-oracle
RUN make build 
WORKDIR /go/src/github.com/judavi/grafeas-oracle/go/v1beta1/main
RUN GO111MODULE=on CGO_ENABLED=1 go build -o grafeas-server .

FROM oraclelinux:7-slim
WORKDIR /
COPY --from=0 /go/src/github.com/judavi/grafeas-oracle/go/v1beta1/main/grafeas-server /grafeas-server
EXPOSE 8080
ARG release=19
ARG update=3

RUN  yum -y install oracle-release-el7 && yum-config-manager --enable ol7_oracle_instantclient && \
     yum -y install oracle-instantclient${release}.${update}-basic oracle-instantclient${release}.${update}-devel oracle-instantclient${release}.${update}-sqlplus && \
     rm -rf /var/cache/yum
     
ENTRYPOINT ["/grafeas-server"]