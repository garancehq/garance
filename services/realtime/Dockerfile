FROM elixir:1.18-alpine AS builder
RUN apk add --no-cache build-base git
WORKDIR /app
ENV MIX_ENV=prod

COPY mix.exs mix.lock ./
RUN mix local.hex --force && mix local.rebar --force
RUN mix deps.get --only prod && mix deps.compile

COPY lib lib
COPY config config
RUN mix compile && mix release

FROM alpine:3.20
RUN apk add --no-cache libstdc++ openssl ncurses-libs
COPY --from=builder /app/_build/prod/rel/realtime /app

ENV PORT=4003
EXPOSE 4003
CMD ["/app/bin/realtime", "start"]
