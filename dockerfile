FROM filvenus/venus-buildenv AS buildenv

COPY . ./venus-market
RUN export GOPROXY=https://goproxy.cn && cd venus-market  && make deps && make
RUN cd venus-market && ldd ./venus-market


FROM filvenus/venus-runtime

# DIR for app
WORKDIR /app

# copy the app from build env
COPY --from=buildenv  /go/venus-market/venus-market /app/venus-market
COPY ./docker/script  /script
COPY ./docker /docker


EXPOSE 41235 58418
ENTRYPOINT ["/script/init.sh"]
