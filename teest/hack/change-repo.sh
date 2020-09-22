#!/bin/sh

PREDEFINED=""
DEFAULT="kedacore"

print_help () {
    echo "Changes repository name in necessary files

-h: print help
-p: change repository to predefined value
-k: change repository to \"kedacore\""
}

declare -a FILES=("deploy/operator.yaml" "Makefile")

while getopts ":hpk" arg; do
  case $arg in
    h)
	    print_help
	    exit 0
	    ;;
    p)
      for FILE in "${FILES[@]}"; do
	      sed -i 's/'"$DEFAULT"'/'"$PREDEFINED"'/g' "$FILE"
      done
      echo "Changed to $PREDEFINED"
	    ;;
    k)
      for FILE in "${FILES[@]}"; do
	      sed -i 's/'"$PREDEFINED"'/'"$DEFAULT"'/g' "$FILE"
      done
      echo "Changed to $DEFAULT"
	    ;;
    *)
        print_help
        exit 0
        ;;
  esac
done
