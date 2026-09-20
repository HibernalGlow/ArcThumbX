/*
 * Entry point for the .appex binary.
 *
 * An app extension has no `@main`: the system starts the process, reads
 * `NSExtension` from Info.plist, and hands control to the principal class.
 * Xcode generates this one-line `main` for you; building with `swiftc`
 * needs it spelled out, or the link fails on `_main`.
 *
 * Foundation exports `_NSExtensionMain` but does not declare it in any
 * public macOS header, hence the explicit prototype.
 */

extern int NSExtensionMain(void);

int main(void) {
    return NSExtensionMain();
}
