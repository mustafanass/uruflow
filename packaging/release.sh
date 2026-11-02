#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

for tool in go node zig tar gzip shasum; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    printf 'Missing release build tool: %s\n' "$tool" >&2
    exit 1
  fi
done

version=$(sed -n 's/^const Current = "\([^"]*\)"$/\1/p' internal/version/version.go)
frontend_version=$(node -p 'JSON.parse(require("node:fs").readFileSync("frontend/package.json", "utf8")).version')
if [ -z "$version" ] || [ "$version" != "$frontend_version" ]; then
  printf 'Server and frontend versions must match.\n' >&2
  exit 1
fi
if [ ! -s internal/web/assets/index.html ]; then
  printf 'Build the frontend first with make frontend.\n' >&2
  exit 1
fi

mkdir -p dist
staging=$(mktemp -d "$(pwd)/dist/.release.XXXXXX")
trap 'rm -rf "$staging"' EXIT
mkdir -p "$staging/assets"

for arch in amd64 arm64; do
  case "$arch" in
    amd64) target=x86_64-linux-musl ;;
    arm64) target=aarch64-linux-musl ;;
  esac
  printf 'Building URUFLOW %s for linux/%s\n' "$version" "$arch"
  for binary in uruflow uruflow-agent; do
    case "$binary" in
      uruflow) package=./cmd/uruflow-server ;;
      uruflow-agent) package=./cmd/uruflow-agent ;;
    esac
    artifact="$binary-$version-linux-$arch"
    bundle="$staging/$artifact"
    mkdir -p "$bundle/packaging/systemd"
    cp LICENSE "$bundle/"
    cp "packaging/systemd/$binary.service" "$bundle/packaging/systemd/"
    CGO_ENABLED=1 GOOS=linux GOARCH="$arch" CC="zig cc -target $target" \
      go build -trimpath -buildvcs=false \
      -tags netgo,osusergo,sqlite_omit_load_extension \
      -ldflags '-s -w -linkmode external -extldflags=-static' \
      -o "$bundle/$binary" "$package"
    COPYFILE_DISABLE=1 tar -C "$bundle" -cf "$staging/bundle.tar" "$binary" LICENSE packaging
    gzip -n -c "$staging/bundle.tar" > "$staging/assets/$artifact.tar.gz"
  done
done

(
  cd "$staging/assets"
  shasum -a 256 uruflow-* > SHA256SUMS.txt
  shasum -a 256 -c SHA256SUMS.txt
)

destination="dist/v$version"
mkdir -p "$destination"
cp "$staging/assets/"* "$destination/"
for arch in amd64 arm64; do
  rm -f "$destination/uruflow-$version-linux-$arch" "$destination/uruflow-agent-$version-linux-$arch"
done
printf 'Release assets: %s\n' "$destination"
