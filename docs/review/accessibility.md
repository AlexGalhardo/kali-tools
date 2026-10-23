# Accessibility review

This note records what is already tested and what a person still has to accept. It does not check the R02 accessibility child.

## Already tested

- English and PT-BR catalog keys match, and every domain error code and capability state has a UI label. That is recorded in the B06 evidence.
- Playwright runs axe-core on Overview, Interfaces, Diagnostics, Wi-Fi, Packet inspector, Discovery & tools, History and Settings. The test fails on a serious or critical violation. Those runs use the browser fixture. They do not prove the native WebView.

## Still required

A person uses the keyboard only, in English and in PT-BR, on the desktop app for each supported OS:

- Move through the sidebar and every screen without a pointer.
- Open and close each dialog, including settings and a reveal confirmation, from the keyboard.
- Confirm focus is visible and that a control's name is the same language as the rest of the screen.
- Confirm the axe result above is not treated as that acceptance.

Record the OS, the archive name and any control that cannot be reached. Until that note exists, this child stays open.
