FROM golang:1.24.4-alpine

RUN apk add --no-cache make

WORKDIR /ms_app

COPY . .

# RUN if [ ! -f "go.mod" ]; then \
#         make init; \
#     fi

# RUN make build

RUN chmod +x /ms_app/app/*

CMD ["sh", "-c", "/ms_app/app/ms_app"]
