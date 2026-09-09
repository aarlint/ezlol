'use strict'
// electron-builder afterSign hook.
//
// When a build has no Developer ID identity (no CSC_LINK in CI, or a local
// build), electron-builder leaves the bundle with Electron's linker-only
// signature and no resource seal. A downloaded copy of that fails Gatekeeper
// with "ezlol is damaged and can't be opened" instead of the usual
// "unidentified developer" prompt. A proper ad-hoc signature fixes that: the
// app then opens via right-click → Open (or after `xattr -cr`).
//
// Builds signed with a real identity are left untouched.
const { spawnSync } = require('node:child_process')
const path = require('node:path')

exports.default = async function adhocSign(context) {
  if (context.electronPlatformName !== 'darwin') return
  const appPath = path.join(context.appOutDir, `${context.packager.appInfo.productFilename}.app`)
  if (process.env.CSC_LINK || process.env.CSC_NAME) return
  const probe = spawnSync('codesign', ['-dv', appPath], { encoding: 'utf8' })
  if (/Authority=Developer ID Application/.test(`${probe.stdout}${probe.stderr}`)) return
  const res = spawnSync('codesign', ['--force', '--deep', '--sign', '-', appPath], { stdio: 'inherit' })
  if (res.status !== 0) throw new Error(`ad-hoc codesign failed with status ${res.status}`)
  console.log(`  • ad-hoc signed ${path.basename(appPath)} (no Developer ID identity in this build)`)
}
