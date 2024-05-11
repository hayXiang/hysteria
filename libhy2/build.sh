export ANDROID_NDK_HOME=/workspaces/android-ndk-r21e
export QUIC_GO_DISABLE_ECN=true

#################### ARMv7 ############
export GOARM=7
export GOARCH=arm
export GOOS=android
export CGO_ENABLED=1
export CC=$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/armv7a-linux-androideabi21-clang
rm -rf libhy2_${GOARCH}.h libhy2_${GOARCH}.so
go build -o .
exit 0
##################### x86-windwos ###############
#rm -rf hy2.dll hy2.h
#CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o hy2.dll -buildmode=c-shared .

#################### x86-linux ############
export GOARCH=amd64
export GOOS=linux
export CGO_ENABLED=1
export CC=$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/clang
rm -rf libhy2_${GOARCH}_$GOOS.h libhy2_${GOARCH}_$GOOS.so
go build -o libhy2_$GOARCH_$GOOS.so -buildmode=c-shared .

#################### x86-android ############
#export GOARCH=amd64
#export GOOS=android
#export CGO_ENABLED=1
#export CC=$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/x86_64-linux-android21-clang
#rm -rf libhy2_${GOARCH}_$GOOS.h libhy2_${GOARCH}_$GOOS.so
#go build -o libhy2_$GOARCH_$GOOS.so -buildmode=c-shared .

#################### ARMv7 ############
export GOARM=7
export GOARCH=arm
export GOOS=android
export CGO_ENABLED=1
export CC=$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/armv7a-linux-androideabi21-clang
rm -rf libhy2_${GOARCH}.h libhy2_${GOARCH}.so
go build -o libhy2_$GOARCH.so -buildmode=c-shared .

#################### ARM64-android ############
export GOARCH=arm64
export GOOS=android
export CGO_ENABLED=1
export CC=$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang
rm -rf libhy2_${GOARCH}.h libhy2_${GOARCH}.so
go build -o libhy2_$GOARCH.so -buildmode=c-shared .