#!/usr/bin/env bash
for WORKER in telegram instagram
do
  cd ${WORKER}
  echo ""
  echo -e "\033[1;34m Compiling ${WORKER} \033[0m"
  make
  cp ../ca-certificates.crt .
  cd ..
done

#cp key.json hashtag/