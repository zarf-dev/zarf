set -e

make clean
make build
make build-examples

for f in hack/examples-checksums/*.txt
do 
  NAME=$(basename $f .txt)
  CHECKSUM=$(tar Oxf build/$NAME.tar.zst checksums.txt | grep -v sboms.tar)
  EXPECTED_CHECKSUM=$(cat $f | grep -v sboms.tar)
  if [[ "$CHECKSUM" != "$EXPECTED_CHECKSUM" ]]
  then
    echo "Package $f does not have expected checksum."
    echo "$CHECKSUM"
    echo "-----"
    echo "$EXPECTED_CHECKSUM"
    exit 1
  fi
done
