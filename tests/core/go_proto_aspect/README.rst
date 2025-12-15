go_proto_library _go_proto_aspect
==========================

_go_proto_aspect test
--------------

Checks that the ``_go_proto_aspect`` used by ``go_proto_library`` works without assuming the ``deps`` are either ``go_library``
or ``go_proto_library`` targets. The test uses a custom code generator rule that provides ``GoInfo``, which is the only requirement
of ``go_proto_library``'s ``deps``, but it doesn't have all the attributes of ``go_library`` or ``go_proto_library``.
