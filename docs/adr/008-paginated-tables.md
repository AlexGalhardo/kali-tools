# ADR 008: Paginated tables for 1.0

Status: accepted on 2026-10-23 by the repository owner (Alex Galhardo).

Context: A08 asked for virtualized packet and result tables. Packet and discovery tables already render at most 100 rows per page. Discovery pagination is tested with 205 synthetic endpoints. Packet import is capped at 10,000 packets.

Decision: pagination satisfies the virtualized-table requirement for 1.0. Keep the existing 100-rows-per-page pagination. Do not add a virtualization library.

Rationale: the behavior is already implemented and tested. Paginated tables keep screen-reader, keyboard and find-in-page access intact for the 1.0 accessibility acceptance.

Consequences: no virtualization dependency is added. Permission and error acceptance for A08 stays a separate open item.

Revisit after 1.0, and only for a specific table, if live capture needs continuous scrolling over large row counts.
