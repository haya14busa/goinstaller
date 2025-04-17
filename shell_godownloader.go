package main

import (
	"fmt"
)

func processGodownloader(repo, path, filename string) ([]byte, error) {
	cfg, err := Load(repo, path, filename)
	if err != nil {
		return nil, fmt.Errorf("unable to parse: %s", err)
	}
	// We only handle the first archive.
	if len(cfg.Archives) == 0 {
		return nil, fmt.Errorf("no archives found in configuration")
	}

	archive := cfg.Archives[0]

	// get archive name template
	archName, err := makeName("", archive.NameTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable generate archive name: %s", err)
	}

	// Store the modified name template back to the archive
	archive.NameTemplate = "NAME=" + archName

	// get checksum name template
	checkName, err := makeName("", cfg.Checksum.NameTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable generate checksum name: %s", err)
	}

	// Store the modified checksum name template
	cfg.Checksum.NameTemplate = "CHECKSUM=" + checkName
	if err != nil {
		return nil, fmt.Errorf("unable generate checksum name: %s", err)
	}

	return makeShell(shellGodownloader, cfg)
}

// nolint: lll
const shellGodownloader = `#!/bin/sh
set -e
# Code generated by godownloader on {{ timestamp }}. DO NOT EDIT.
#

usage() {
  this=$1
  cat <<EOF
$this: download go binaries for {{ $.Release.GitHub.Owner }}/{{ $.Release.GitHub.Name }}

Usage: $this [-b] bindir [-d] [tag]
  -b sets bindir or installation directory, Defaults to ./bin
  -d turns on debug logging
   [tag] is a tag from
   https://github.com/{{ $.Release.GitHub.Owner }}/{{ $.Release.GitHub.Name }}/releases
   If tag is missing, then the latest will be used.

 Generated by godownloader
  https://github.com/goreleaser/godownloader

EOF
  exit 2
}

parse_args() {
  #BINDIR is ./bin unless set be ENV
  # over-ridden by flag below

  BINDIR=${BINDIR:-./bin}
  while getopts "b:dh?x" arg; do
    case "$arg" in
      b) BINDIR="$OPTARG" ;;
      d) log_set_priority 10 ;;
      h | \?) usage "$0" ;;
      x) set -x ;;
    esac
  done
  shift $((OPTIND - 1))
  TAG=$1
}
# this function wraps all the destructive operations
# if a curl|bash cuts off the end of the script due to
# network, either nothing will happen or will syntax error
# out preventing half-done work
execute() {
  tmpdir=$(mktemp -d)
  log_debug "downloading files into ${tmpdir}"
  http_download "${tmpdir}/${TARBALL}" "${TARBALL_URL}"
  http_download "${tmpdir}/${CHECKSUM}" "${CHECKSUM_URL}"
  hash_sha256_verify "${tmpdir}/${TARBALL}" "${tmpdir}/${CHECKSUM}"
  {{- if (index .Archives 0).WrapInDirectory }}
  srcdir="${tmpdir}/${NAME}"
  rm -rf "${srcdir}"
  {{- else }}
  srcdir="${tmpdir}"
  {{- end }}
  (cd "${tmpdir}" && untar "${TARBALL}")
  test ! -d "${BINDIR}" && install -d "${BINDIR}"
  for binexe in $BINARIES; do
    if [ "$OS" = "windows" ]; then
      binexe="${binexe}.exe"
    fi
    install "${srcdir}/${binexe}" "${BINDIR}/"
    log_info "installed ${BINDIR}/${binexe}"
  done
  rm -rf "${tmpdir}"
}
get_binaries() {
  case "$PLATFORM" in
  {{- range $platform, $binaries := (platformBinaries .) }}
    {{ $platform }}) BINARIES="{{ join $binaries " " }}" ;;
  {{- end }}
    *)
      log_crit "platform $PLATFORM is not supported.  Make sure this script is up-to-date and file request at https://github.com/${PREFIX}/issues/new"
      exit 1
      ;;
  esac
}
tag_to_version() {
  if [ -z "${TAG}" ]; then
    log_info "checking GitHub for latest tag"
  else
    log_info "checking GitHub for tag '${TAG}'"
  fi
  REALTAG=$(github_release "$OWNER/$REPO" "${TAG}") && true
  if test -z "$REALTAG"; then
    log_crit "unable to find '${TAG}' - use 'latest' or see https://github.com/${PREFIX}/releases for details"
    exit 1
  fi
  # if version starts with 'v', remove it
  TAG="$REALTAG"
  VERSION=${TAG#v}
}
adjust_format() {
  # change format (tar.gz or zip) based on OS
  {{- with (index .Archives 0).FormatOverrides }}
  case ${OS} in
  {{- range . }}
    {{- if .Format }}
    {{ .Goos }}) FORMAT={{ .Format }} ;;
    {{- else if .Formats }}
    {{ .Goos }}) FORMAT={{ index .Formats 0 }} ;;
    {{- end }}
 {{- end }}
  esac
  {{- end }}
  true
}
adjust_os() {
  # adjust archive name based on OS
  {{- if or (contains (index .Archives 0).NameTemplate "title .Os") (contains (index .Archives 0).NameTemplate "title .OS") }}
  # This archive uses title case for OS names
  case ${OS} in
    darwin) OS=Darwin ;;
    linux) OS=Linux ;;
    windows) OS=Windows ;;
  esac
  {{- else }}
  # This archive uses lowercase OS names
  # No need to adjust OS names
  {{- end }}
  true
}
adjust_arch() {
  # adjust archive name based on ARCH and whether the template uses x86_64
  {{- if contains (index .Archives 0).NameTemplate "x86_64" }}
  # This archive uses x86_64 for amd64
  case ${ARCH} in
    amd64) ARCH=x86_64 ;;
    386) ARCH=i386 ;;
  esac
  {{- else if contains (index .Archives 0).NameTemplate "i386" }}
  # This archive uses i386 for 386
  case ${ARCH} in
    386) ARCH=i386 ;;
  esac
  {{- else }}
  # No need to adjust ARCH names
  {{- end }}
  true
}
` + shellfn + `
PROJECT_NAME="{{ $.ProjectName }}"
OWNER={{ $.Release.GitHub.Owner }}
REPO="{{ $.Release.GitHub.Name }}"
BINARY={{ (index .Builds 0).Binary }}
FORMAT=tar.gz
OS=$(uname_os)
ARCH=$(uname_arch)
PREFIX="$OWNER/$REPO"

# use in logging routines
log_prefix() {
	echo "$PREFIX"
}
PLATFORM="${OS}/${ARCH}"
GITHUB_DOWNLOAD=https://github.com/${OWNER}/${REPO}/releases/download

uname_os_check "$OS"
uname_arch_check "$ARCH"

parse_args "$@"

get_binaries

tag_to_version

adjust_format

adjust_os

adjust_arch

log_info "found version: ${VERSION} for ${TAG}/${OS}/${ARCH}"
{{ evaluateNameTemplate (index .Archives 0).NameTemplate }}
TARBALL=${NAME}.${FORMAT}
TARBALL_URL=${GITHUB_DOWNLOAD}/${TAG}/${TARBALL}
{{ .Checksum.NameTemplate }}
CHECKSUM_URL=${GITHUB_DOWNLOAD}/${TAG}/${CHECKSUM}



execute
`
