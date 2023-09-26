export ANDROID_HOME="${HOME}/.local/storage/Android/Sdk/"
export ANDROID_NDK_HOME="${HOME}/.local/storage/Android/android-ndk-r23c"
export PATH=$PATH:"${HOME}/.local/share/JetBrains/Toolbox/apps/android-studio/jbr/bin"


gomobile bind -ldflags='-s -w -buildid=' -trimpath -target="android/arm64,android/amd64" -androidapi 21 -o appctr.aar -v .
