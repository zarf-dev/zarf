#!/bin/bash

# Assumptions:
# - run as root (for proper zarf permissions)

# env
ME="$( basename "${BASH_SOURCE[0]}" )"
HERE="$( cd "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
SCENARIOS="$HERE/scenarios"
RECORDINGS="$HERE/recordings"
ROOTDIR="$( realpath "$HERE/../.." )"
ZARFDIR="$( realpath "$ROOTDIR/build" )"

# func defs
function help() {
  echo ""
  echo "Usage: $ME <run|pub|see|sub|zap> [args...]"
  echo ""
}

function run() {
  local fname="${1##*/}"
  local fpath="$SCENARIOS/$fname"

  if [ ! -f "$fpath" ] ; then
    echo ""
    echo "Can't find scenario: $fpath"
    echo ""
    return 1
  fi

  mkdir -p "$RECORDINGS"
  local base="${fname/.exp/}"
  local recording="$RECORDINGS/$base.rec"
  local log="${recording/.rec/.log}"
  rm -f "$RECORDINGS/$base".*

  local cols=$(tput cols)
  local rows=$(tput lines)
  (
    cd "$ZARFDIR" \
      && stty cols 90 rows 30 \
      && expect -d -f "$fpath" "$recording" "$log"
  )
  stty cols "$cols" rows "$rows"
}

function run_all() {
  local scenarios=$(find "$SCENARIOS" -name '*.exp' -type f)

  # need to keep access to calling tty so settings can be
  #   manipulated during recording
  local tty="$(tty)"
  echo "$scenarios" | while read scenario ; do
    run "$(basename "$scenario")" < "$tty"
  done
}

function pub() {
  local fname="${1##*/}"
  local fpath="$RECORDINGS/$fname"

  if [ ! -f "$fpath" ] ; then
    echo ""
    echo "Can't find recording: $fpath"
    echo ""
    return 1
  fi

  local uname="$( basename $fname ".rec" ).url"
  local upath="$RECORDINGS/$uname"
  if [ -f "$upath" ] ; then
    echo "URL file found. Skipping: $fname." && return 0
  fi

  asciinema upload "$fpath" \
    | grep -Po 'https://asciinema.org[^ |\n]*' \
    | tr -d '\n' \
    > "$upath"
}

function pub_all() {
  local recordings=$(find "$RECORDINGS" -name '*.rec' -type f)

  echo "$recordings" | while read recording ; do
    pub "$(basename "$recording")"
  done
}

function see() {
  local found="$(
    find "$ROOTDIR" -type f -name '*.md' -print0 \
      | xargs -0 grep -Pon '(http|https)://asciinema.org/a/[^ \]\)'"'"'"]*'
  )"
  if [ -z "$found" ] ; then return 0 ; fi

  echo "$found" | while IFS= read -r f ; do
    local file=$(echo "$f" | awk -F ':' '{print $1}' )
    local line=$(echo "$f" | awk -F ':' '{print $2}' )
    local url=$(echo "$f" | cut -d ':' -f3- )
    local proto=$(echo "$url" | cut -d ':' -f1 )
    local host=$(echo "$url" | cut -d '/' -f3 )
    local path=$(echo "$url" \
      | cut -d '/' -f4- \
      | cut -d '#' -f1 \
      | cut -d '?' -f1 \
      | cut -d '.' -f1
    )
    local ext=$(echo "$url" \
      | cut -d '#' -f1 \
      | cut -d '?' -f1 \
      | awk -F '/' '{print $NF}' \
      | grep '\.' \
      | cut -d '.' -f2
    )
    local query=$(echo "$url" | awk -F '?' '{print $2}' | grep -o '^[^#]*' )
    local fragment=$(echo "$url" | awk -F '#' '{print $2}' )
    local scenario=$(echo "$query" | grep -Po 'x-scenario=\K[^&]*' )

    local sub="no"
    local rsu="$RECORDINGS/$scenario.url"
    if [ -f "$rsu" ] ; then
      local found_base="$proto://$host/$path"
      local rsu_url=$( cat "$rsu" )
      if [ "$found_base" != "$rsu_url" ] ; then
        local from="$url"
        local to="$rsu_url${ext:+.$ext}${query:+?$query}${fragment:+#$fragment}"
        sub="$from > $to"
      fi
    fi

    echo "Found: $f"
    echo " file     : $file"
    echo " line     : $line"
    echo " protocol : $proto"
    echo " host     : $host"
    echo " path     : $path"
    echo " ext      : $ext"
    echo " query    : $query"
    echo " fragment : $fragment"
    echo " scenario : $scenario"
    echo " sub      : $sub"
    echo ""
  done
}

function sub() {
  local lines="$( printf '%.0s- ' {1..12} )"
  local found="$( cat - | paste -d '\t' $lines )" # <-- reads stdin!
  local clean="$( echo "$found" | tr -s ' ' | sed -E 's/: \t/: -\t/g' )"
  
  echo "$clean" | while IFS= read -r c ; do
    local  sub="$( echo "$c" | grep -Po 'sub : \K[^\t]*' )"
    if [ "$sub" = "no" ] ; then continue ; fi

    local file="$( echo "$c" | grep -Po 'file : \K[^\t]*' )"
    local line="$( echo "$c" | grep -Po 'line : \K[^\t]*' )"
    local from="$( echo "$sub" | awk -F '>' '{print $1}' | tr -d ' ' )"
    local   to="$( echo "$sub" | awk -F '>' '{print $2}' | tr -d ' ' )"

    local to_sed_safe="${to/&/\\&}"
    sed -i'' "${line}s|$from|$to_sed_safe|" "$file"
  done
}

function zap() {
  # Web scraping, eeewww!
  local account="$( cat "$HERE/asciinema-org" )"

  # scrape paging controls
  local page_one="$( curl --silent --location "$account" )"
  local page_nav="$( echo "$page_one" \
    | grep '<nav aria-label="Page navigation">'
  )"

  local page_max="1"
  if [ -n "$page_nav" ] ; then
    page_max="$( echo "$page_nav" \
      | grep -Po 'href="\?page=[^"]*"' \
      | tail -n2 | head -n1 \
      | awk -F '"' '{print $2}' \
      | awk -F '=' '{print $2}'
    )"
  fi

  # crawl pages & collect recording links
  local recordings
  for i in $( eval "echo {1..$page_max}" ) ; do
    local body="$( curl --location "$account?page=$i")"
    local links="$( echo "$body" | grep -Po '<a href="/a/.*/a>' )"
    local times="$( echo "$body" | grep -Po '<time datetime=".*"' )"
    local dirty="$( paste <(echo "$links") <(echo "$times") )"

    local link="$( echo "$dirty" | sed -E 's|^.*href="([^"]*)".*$|\1|' )"
    local title="$( echo "$dirty" | sed -E 's|^.*>([^<]*)</a.*$|\1|' )"
    local tstamp="$( echo "$dirty" | sed -E 's|^.*datetime="([^"]*)".*$|\1|' )"
    local clean="$(
      paste -d '\t' -- <(echo "$tstamp") <(echo "$link") <(echo "$title")
    )"

    recordings="$recordings$clean"
    if [ ! "$i" = "$page_max" ] ; then recordings="$recordings"$'\n' ; fi
  done

  # sort links newest first
  recordings="$( echo "$recordings" | sort -t$'\t' -k1,1 -nr )"

  # mark all-but newest for deletion
  local plan
  local title_cache
  while read r ; do
    local title="$( echo "$r" | awk -F $'\t' '{print $3}' )"
    local verdict="keep"
    if [ "$title" != "$title_cache" ] ; then
      title_cache="$title"
    else
      verdict="delete"
    fi

    plan="$plan$r"$'\t'"$verdict"$'\n'
  done <<< "$recordings"
  plan="${plan::-1}"
  
  # act on the plan?
  echo ""
  echo "$plan"
  echo ""
  read -p "Do the deletes now (y/N)? " ans ; echo ""
  if [ ! "${ans,,}" = "y" ] ; then return 0 ; fi

  # act on the plan!
  echo "Login to $account & open the dev tools. Paste in your:" ; echo ""
  read -p "    auth_token cookie: " auth_token
  read -p "_asciinema_key cookie: " _asciinema_key
  echo ""
  if [ -z "$auth_token" ] || [ -z "$_asciinema_key" ] ; then
    echo "Can't continue without all the magic strings! Quitting."
    echo ""
    return 1
  fi

  echo "$plan" | grep -P 'delete$' | while IFS= read -r p ; do
    local path="$( echo "$p" | awk -F $'\t' '{print $2}' )"
    
    # secure a csrf token via "get" request
    local get="$( cat "$HERE/get.tmpl.curl" \
      | sed 's|{{path}}|'"$path"'|g' \
      | sed 's|{{account}}|'"$account"'|g' \
      | sed 's|{{auth_token}}|'"$auth_token"'|g' \
      | sed 's|{{_asciinema_key}}|'"$_asciinema_key"'|g'
    )"
    local get_resp="$( eval "$get" )"
    local csrf="$( echo "$get_resp" \
      | grep 'data-csrf=' \
      | sed -E 's|^.*data-csrf="([^"]*).*$|\1|' \
      | head -n 1
    )"

    # send the "delete" request
    local delete="$( cat "$HERE/delete.tmpl.curl" \
      | sed 's|{{path}}|'"$path"'|g' \
      | sed 's|{{auth_token}}|'"$auth_token"'|g' \
      | sed 's|{{_asciinema_key}}|'"$_asciinema_key"'|g' \
      | sed 's|{{csrf}}|'"$csrf"'|g'
    )"
    local delete_resp="$( eval "$delete" )"
  done
}


# args
if [ "$#" -lt 1 ] ; then help && exit ; fi
cmd="$1" ; shift


# main
case "$cmd" in
  "help")
    help && exit 0
    ;;

  "run")
    if [ "$#" -lt 1 ] || [ "$1" = "all" ] ; then run_all ; else run "$@" ; fi
    ;;
  
  "pub")
    if [ "$#" -lt 1 ] || [ "$1" = "all" ] ; then pub_all ; else pub "$@" ; fi
    ;;

  "see") see ;;

  "sub") see | sub ;;

  "zap") zap ;;

  *)
    help && exit 1
    ;;
esac
