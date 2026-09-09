# Signing and notarizing the macOS build

Unsigned builds work, but macOS shows "unidentified developer" and the app has to update itself with its own
swap updater. With a Developer ID certificate the builds are signed, notarized (no Gatekeeper prompt) and use
electron-updater in place. CI (`.github/workflows/build.yml`) does all of it once five repository secrets exist.

## Unsigned builds: "ezlol is damaged and can't be opened"

Builds made without a Developer ID identity are ad-hoc signed by `electron/scripts/adhoc-sign.js`, so macOS
shows the usual "unidentified developer" prompt (right-click the app → **Open** once). If you still see
"damaged and can't be opened, move to Trash" (older builds, or a copy re-quarantined by another app), clear the
quarantine flag and re-seal it:

```bash
xattr -cr /Applications/ezlol.app
codesign --force --deep --sign - /Applications/ezlol.app
```

## One-time setup

1. **Apple Developer Program** membership (https://developer.apple.com/programs/, $99/yr).
2. **Developer ID Application certificate**
   - Keychain Access → Certificate Assistant → *Request a Certificate From a Certificate Authority* → save to disk.
   - https://developer.apple.com/account/resources/certificates → **+** → *Developer ID Application* → upload the
     request → download `developerID_application.cer` → double-click to install it in your login keychain.
   - Keychain Access → My Certificates → right-click "Developer ID Application: <name> (<TEAM>)" → *Export* →
     `.p12` with a password.
3. **App-specific password** for notarization: https://appleid.apple.com → Sign-In and Security →
   App-Specific Passwords → generate one (label it "ezlol notarize").
4. **Team ID**: https://developer.apple.com/account → Membership details (yours shows as `2S4A4V6TJD` on this Mac).

## Repository secrets

Run these locally; each prompts for the value so nothing lands in shell history or chat:

```bash
base64 -i ~/path/to/ezlol-developer-id.p12 | gh secret set CSC_LINK --repo aarlint/ezlol
gh secret set CSC_KEY_PASSWORD --repo aarlint/ezlol            # the .p12 export password
gh secret set APPLE_ID --repo aarlint/ezlol                    # your Apple ID email
gh secret set APPLE_APP_SPECIFIC_PASSWORD --repo aarlint/ezlol # the app-specific password
gh secret set APPLE_TEAM_ID --repo aarlint/ezlol               # e.g. 2S4A4V6TJD
```

Then push a tag. The mac jobs detect `CSC_LINK`, sign with hardened runtime, submit to Apple's notary
service, staple the ticket, and publish `latest-mac.yml` so signed installs update through electron-updater.
The app itself notices it is signed (`Contents/_CodeSignature`) and stops using the swap updater.

## Checking a build

```bash
codesign -dv --verbose=2 /Applications/ezlol.app      # Authority=Developer ID Application: ...
spctl -a -vv /Applications/ezlol.app                  # accepted, source=Notarized Developer ID
```
