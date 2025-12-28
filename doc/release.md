# release

## Сборка релиза

Чтобы собрать релиз с тегом для всех поддерживаемых платформ, в корне проекта, необходимо выполнить:
```sh
make release VERSION=1.0.0
```

Где у флага **VERSION**, указать версию релиза.

После чего, в папке **build/dist/**, будут находиться собранные архивы бинарников для поддерживаемых платформ.

```sh
./build/dist/
├── gogue-1.0.0-darwin-amd64.tar.gz
├── gogue-1.0.0-darwin-arm64.tar.gz
├── gogue-1.0.0-freebsd-amd64.tar.gz
├── gogue-1.0.0-freebsd-arm64.tar.gz
├── gogue-1.0.0-linux-amd64.tar.gz
├── gogue-1.0.0-linux-arm64.tar.gz
├── gogue-1.0.0-windows-386.zip
└── gogue-1.0.0-windows-amd64.zip
```

Если не указать версию релиза, будет собран релиз с версией **dev**.

```sh
./build/dist/
├── gogue-dev-darwin-amd64.tar.gz
├── gogue-dev-darwin-arm64.tar.gz
├── gogue-dev-freebsd-amd64.tar.gz
├── gogue-dev-freebsd-arm64.tar.gz
├── gogue-dev-linux-amd64.tar.gz
├── gogue-dev-linux-arm64.tar.gz
├── gogue-dev-windows-386.zip
└── gogue-dev-windows-amd64.zip
```

## Список поддерживаемых платформ

Поддерживаемые платформы:
- linux/amd64 
- linux/arm64 
- windows/amd64 
- windows/386 
- darwin/amd64 
- darwin/arm64 
- freebsd/amd64
- freebsd/arm64

### Не поддерживаемые платформы*

Для компиляции для **32** битных **ARM** архитектур, использйте исходных код, и флаг **GOARM**, под ваше устройство. [Теги](https://go.dev/wiki/GoArm). 