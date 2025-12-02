Popular repository tests
========================

These tests are designed to check that gazelle and rules_go together can cope
with a list of popluar repositories people depend on.

It helps catch changes that might break a large number of users.

.. contents::

org_golang_x_crypto
___________________

This runs tests from the repository `golang.org/x/crypto <https://golang.org/x/crypto>`_

* @org_golang_x_crypto//acme:acme_test
* @org_golang_x_crypto//acme/autocert:autocert_test
* @org_golang_x_crypto//argon2:argon2_test
* @org_golang_x_crypto//bcrypt:bcrypt_test
* @org_golang_x_crypto//blake2b:blake2b_test
* @org_golang_x_crypto//blake2s:blake2s_test
* @org_golang_x_crypto//blowfish:blowfish_test
* @org_golang_x_crypto//bn256:bn256_test
* @org_golang_x_crypto//cast5:cast5_test
* @org_golang_x_crypto//chacha20:chacha20_test
* @org_golang_x_crypto//chacha20poly1305:chacha20poly1305_test
* @org_golang_x_crypto//cryptobyte:cryptobyte_test
* @org_golang_x_crypto//curve25519:curve25519_test
* @org_golang_x_crypto//ed25519:ed25519_test
* @org_golang_x_crypto//hkdf:hkdf_test
* @org_golang_x_crypto//internal/alias:alias_test
* @org_golang_x_crypto//internal/poly1305:poly1305_test
* @org_golang_x_crypto//md4:md4_test
* @org_golang_x_crypto//nacl/auth:auth_test
* @org_golang_x_crypto//nacl/box:box_test
* @org_golang_x_crypto//nacl/sign:sign_test
* @org_golang_x_crypto//ocsp:ocsp_test
* @org_golang_x_crypto//openpgp:openpgp_test
* @org_golang_x_crypto//openpgp/armor:armor_test
* @org_golang_x_crypto//openpgp/clearsign:clearsign_test
* @org_golang_x_crypto//openpgp/elgamal:elgamal_test
* @org_golang_x_crypto//openpgp/packet:packet_test
* @org_golang_x_crypto//openpgp/s2k:s2k_test
* @org_golang_x_crypto//otr:otr_test
* @org_golang_x_crypto//pbkdf2:pbkdf2_test
* @org_golang_x_crypto//pkcs12:pkcs12_test
* @org_golang_x_crypto//pkcs12/internal/rc2:rc2_test
* @org_golang_x_crypto//ripemd160:ripemd160_test
* @org_golang_x_crypto//salsa20:salsa20_test
* @org_golang_x_crypto//salsa20/salsa:salsa_test
* @org_golang_x_crypto//scrypt:scrypt_test
* @org_golang_x_crypto//sha3:sha3_test
* @org_golang_x_crypto//ssh/internal/bcrypt_pbkdf:bcrypt_pbkdf_test
* @org_golang_x_crypto//ssh/knownhosts:knownhosts_test
* @org_golang_x_crypto//tea:tea_test
* @org_golang_x_crypto//twofish:twofish_test
* @org_golang_x_crypto//x509roots/nss:nss_test
* @org_golang_x_crypto//xtea:xtea_test
* @org_golang_x_crypto//xts:xts_test


org_golang_x_net
________________

This runs tests from the repository `golang.org/x/net <https://golang.org/x/net>`_

* @org_golang_x_net//context/ctxhttp:ctxhttp_test
* @org_golang_x_net//dns/dnsmessage:dnsmessage_test
* @org_golang_x_net//html:html_test
* @org_golang_x_net//html/atom:atom_test
* @org_golang_x_net//http/httpguts:httpguts_test
* @org_golang_x_net//http/httpproxy:httpproxy_test
* @org_golang_x_net//http2/h2c:h2c_test
* @org_golang_x_net//http2/hpack:hpack_test
* @org_golang_x_net//idna:idna_test
* @org_golang_x_net//internal/quic/cmd/interop:interop_test
* @org_golang_x_net//internal/socks:socks_test
* @org_golang_x_net//internal/sockstest:sockstest_test
* @org_golang_x_net//internal/timeseries:timeseries_test
* @org_golang_x_net//ipv4:ipv4_test
* @org_golang_x_net//ipv6:ipv6_test
* @org_golang_x_net//netutil:netutil_test
* @org_golang_x_net//proxy:proxy_test
* @org_golang_x_net//publicsuffix:publicsuffix_test
* @org_golang_x_net//quic:quic_test
* @org_golang_x_net//quic/qlog:qlog_test
* @org_golang_x_net//route:route_test
* @org_golang_x_net//trace:trace_test
* @org_golang_x_net//webdav:webdav_test
* @org_golang_x_net//webdav/internal/xml:xml_test
* @org_golang_x_net//websocket:websocket_test
* @org_golang_x_net//xsrftoken:xsrftoken_test


org_golang_x_sys
________________

This runs tests from the repository `golang.org/x/sys <https://golang.org/x/sys>`_

* @org_golang_x_sys//cpu:cpu_test
* @org_golang_x_sys//execabs:execabs_test
* @org_golang_x_sys//plan9:plan9_test
* @org_golang_x_sys//unix/internal/mkmerge:mkmerge_test
* @org_golang_x_sys//windows/mkwinsyscall:mkwinsyscall_test
* @org_golang_x_sys//windows/registry:registry_test
* @org_golang_x_sys//windows/svc:svc_test
* @org_golang_x_sys//windows/svc/eventlog:eventlog_test
* @org_golang_x_sys//windows/svc/mgr:mgr_test


org_golang_x_text
_________________

This runs tests from the repository `golang.org/x/text <https://golang.org/x/text>`_

* @org_golang_x_text//cases:cases_test
* @org_golang_x_text//collate:collate_test
* @org_golang_x_text//collate/build:build_test
* @org_golang_x_text//currency:currency_test
* @org_golang_x_text//date:date_test
* @org_golang_x_text//encoding:encoding_test
* @org_golang_x_text//encoding/htmlindex:htmlindex_test
* @org_golang_x_text//encoding/ianaindex:ianaindex_test
* @org_golang_x_text//feature/plural:plural_test
* @org_golang_x_text//internal:internal_test
* @org_golang_x_text//internal/catmsg:catmsg_test
* @org_golang_x_text//internal/colltab:colltab_test
* @org_golang_x_text//internal/export/idna:idna_test
* @org_golang_x_text//internal/export/unicode:unicode_test
* @org_golang_x_text//internal/format:format_test
* @org_golang_x_text//internal/language:language_test
* @org_golang_x_text//internal/language/compact:compact_test
* @org_golang_x_text//internal/number:number_test
* @org_golang_x_text//internal/stringset:stringset_test
* @org_golang_x_text//internal/tag:tag_test
* @org_golang_x_text//internal/triegen:triegen_test
* @org_golang_x_text//internal/ucd:ucd_test
* @org_golang_x_text//language:language_test
* @org_golang_x_text//language/display:display_test
* @org_golang_x_text//message:message_test
* @org_golang_x_text//message/catalog:catalog_test
* @org_golang_x_text//number:number_test
* @org_golang_x_text//runes:runes_test
* @org_golang_x_text//search:search_test
* @org_golang_x_text//secure/bidirule:bidirule_test
* @org_golang_x_text//secure/precis:precis_test
* @org_golang_x_text//transform:transform_test
* @org_golang_x_text//unicode/bidi:bidi_test
* @org_golang_x_text//unicode/cldr:cldr_test
* @org_golang_x_text//unicode/norm:norm_test
* @org_golang_x_text//unicode/rangetable:rangetable_test
* @org_golang_x_text//unicode/runenames:runenames_test
* @org_golang_x_text//width:width_test


org_golang_x_tools
__________________

This runs tests from the repository `golang.org/x/tools <https://golang.org/x/tools>`_

* @org_golang_x_tools//benchmark/parse:parse_test
* @org_golang_x_tools//cmd/benchcmp:benchcmp_test
* @org_golang_x_tools//cmd/bisect:bisect_test
* @org_golang_x_tools//cmd/digraph:digraph_test
* @org_golang_x_tools//cmd/go-contrib-init:go-contrib-init_test
* @org_golang_x_tools//cmd/splitdwarf/internal/macho:macho_test
* @org_golang_x_tools//cover:cover_test
* @org_golang_x_tools//go/analysis:analysis_test
* @org_golang_x_tools//go/analysis/passes/directive/testdata/src/a:a_test
* @org_golang_x_tools//go/analysis/passes/internal/analysisutil:analysisutil_test
* @org_golang_x_tools//go/ast/astutil:astutil_test
* @org_golang_x_tools//go/callgraph:callgraph_test
* @org_golang_x_tools//go/callgraph/vta/internal/trie:trie_test
* @org_golang_x_tools//godoc/redirect:redirect_test
* @org_golang_x_tools//godoc/vfs:vfs_test
* @org_golang_x_tools//godoc/vfs/gatefs:gatefs_test
* @org_golang_x_tools//godoc/vfs/mapfs:mapfs_test
* @org_golang_x_tools//internal/aliases:aliases_test
* @org_golang_x_tools//internal/bisect:bisect_test
* @org_golang_x_tools//internal/diff:diff_test
* @org_golang_x_tools//internal/diff/lcs:lcs_test
* @org_golang_x_tools//internal/diff/myers:myers_test
* @org_golang_x_tools//internal/edit:edit_test
* @org_golang_x_tools//internal/event:event_test
* @org_golang_x_tools//internal/event/export:export_test
* @org_golang_x_tools//internal/event/export/ocagent:ocagent_test
* @org_golang_x_tools//internal/event/export/ocagent/wire:wire_test
* @org_golang_x_tools//internal/event/keys:keys_test
* @org_golang_x_tools//internal/event/label:label_test
* @org_golang_x_tools//internal/gopathwalk:gopathwalk_test
* @org_golang_x_tools//internal/jsonrpc2:jsonrpc2_test
* @org_golang_x_tools//internal/jsonrpc2/servertest:servertest_test
* @org_golang_x_tools//internal/jsonrpc2_v2:jsonrpc2_v2_test
* @org_golang_x_tools//internal/memoize:memoize_test
* @org_golang_x_tools//internal/modindex:modindex_test
* @org_golang_x_tools//internal/packagestest/testdata:testdata_test
* @org_golang_x_tools//internal/pkgbits:pkgbits_test
* @org_golang_x_tools//internal/proxydir:proxydir_test
* @org_golang_x_tools//internal/robustio:robustio_test
* @org_golang_x_tools//internal/stack:stack_test
* @org_golang_x_tools//internal/tokeninternal:tokeninternal_test
* @org_golang_x_tools//internal/typesinternal:typesinternal_test
* @org_golang_x_tools//playground/socket:socket_test
* @org_golang_x_tools//refactor/satisfy:satisfy_test
* @org_golang_x_tools//txtar:txtar_test


com_github_golang_glog
______________________

This runs tests from the repository `github.com/golang/glog <https://github.com/golang/glog>`_

* @com_github_golang_glog//:glog_test


org_golang_x_sync
_________________

This runs tests from the repository `golang.org/x/sync <https://golang.org/x/sync>`_

* @org_golang_x_sync//errgroup:errgroup_test
* @org_golang_x_sync//semaphore:semaphore_test
* @org_golang_x_sync//singleflight:singleflight_test
* @org_golang_x_sync//syncmap:syncmap_test


org_golang_x_mod
________________

This runs tests from the repository `golang.org/x/mod <https://golang.org/x/mod>`_

* @org_golang_x_mod//modfile:modfile_test
* @org_golang_x_mod//module:module_test
* @org_golang_x_mod//semver:semver_test
* @org_golang_x_mod//sumdb:sumdb_test
* @org_golang_x_mod//sumdb/dirhash:dirhash_test
* @org_golang_x_mod//sumdb/note:note_test
* @org_golang_x_mod//sumdb/storage:storage_test


