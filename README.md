# FixYouTube
Embed YouTube videos on Telegram, Discord and more!

## How to use:
Replace `www.youtube.com` or `youtu.be` with `y.outube.duckdns.org` to fix embeds for short videos.

https://github.com/BiRabittoh/FixYouTube/assets/26506860/e1ad5397-41c8-4073-9b3e-598c66241255

## Instructions (Docker)

### With reverse proxy
Copy the template config file and make your adjustments. My configuration is based on [DuckDNS](http://duckdns.org/) but you can use whatever provider you find [here](https://docs.linuxserver.io/general/swag#docker-compose).

```
cp docker/swag.env.example docker/swag.env
nano docker/swag.env
```

Finally: `docker-compose up -d`.

### Without reverse proxy
Simply run:
```
docker run -d -p 3000:3000 --name fixyoutube-go --restart unless-stopped ghcr.io/birabittoh/fixyoutube-go:main
```

## Instructions (local)
```
go test -v ./...
go run .
```
