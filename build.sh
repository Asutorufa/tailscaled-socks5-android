#!/bin/bash

JAVA_BIN=

JAVA_BIN="${HOME}/.local/share/JetBrains/Toolbox/apps/android-studio/jbr/bin"

#for file in "${HOME}"/.local/share/JetBrains/Toolbox/apps/AndroidStudio/ch-0/*
#do
#  if [ -f "${file}/jbr/bin/javac" ]; then
#    JAVA_BIN="${file}/jbr/bin"
#    break
#   fi
#done

if [ "${JAVA_BIN}" = "" ]; then
  echo "can't find java bin"
  exit 1
fi

PATH=$PATH:${JAVA_BIN} KEYSTORE_PATH=/home/asutorufa/Documents/Programming/asutorufa.keystore KEY_ALIAS=key0 KEYSTORE_PASSWORD=asutorufa KEY_PASSWORD=asutorufa ./gradlew app:assembleRelease --stacktrace

