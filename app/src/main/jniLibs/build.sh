set -x
current=$(dirname "$(readlink -f "$0")")
CORE_HOME=${CORE_HOME:-${HOME}/Documents/Programming/tailscale}
cd $CORE_HOME
if [ "${ANDROID_NDK_HOME}" = "" ]; then
	export ANDROID_NDK_HOME="/opt/android-ndk"
fi

export GOOS=android
export CGO_ENABLED=1
## see: https://tailscale.com/kb/1207/small-tailscale
BUILD_TAGS="ts_include_cli,ts_omit_aws,ts_omit_bird,ts_omit_tap,ts_omit_kube,ts_omit_completion"
CC=${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang GOARCH=arm64 go build -ldflags='-s -w -checklinkname=0 -buildid=' -tags ${BUILD_TAGS} -trimpath -o ${current}/arm64-v8a/libtailscaled.so -v ./cmd/tailscaled/.
CC=${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/linux-x86_64/bin/x86_64-linux-android21-clang GOARCH=amd64 go build -ldflags='-s -w -checklinkname=0 -buildid=' -tags ${BUILD_TAGS} -trimpath -o ${current}/x86_64/libtailscaled.so -v ./cmd/tailscaled/.

# CC=${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang GOARCH=arm64 go build -ldflags='-s -w -buildid=' -trimpath -o ${current}/arm64-v8a/libtailscale.so -v ./cmd/tailscale/.
# CC=${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/linux-x86_64/bin/x86_64-linux-android21-clang GOARCH=amd64 go build -ldflags='-s -w -buildid=' -trimpath -o ${current}/x86_64/libtailscale.so -v ./cmd/tailscale/.
