# Homebrew Tap Setup: indrasvat/homebrew-tap

This document describes how to set up and maintain the Homebrew tap for nidhi.

## Repository Structure

The Homebrew tap lives at `github.com/indrasvat/homebrew-tap`:

```
homebrew-tap/
├── Formula/
│   └── nidhi.rb      # Homebrew formula (auto-updated by goreleaser)
└── README.md
```

## Creating the Tap Repository

### Step 1: Create the repo

```bash
# Create the homebrew-tap repo on GitHub.
gh repo create indrasvat/homebrew-tap --public --description "Homebrew formulae for indrasvat's tools"

# Clone and initialize.
git clone https://github.com/indrasvat/homebrew-tap.git
cd homebrew-tap
mkdir -p Formula
```

### Step 2: Create the initial formula

goreleaser auto-generates `Formula/nidhi.rb` on each release. For the initial
setup, create a placeholder:

```ruby
# Formula/nidhi.rb
# This formula is auto-updated by goreleaser on each release.
# Manual edits will be overwritten.

class Nidhi < Formula
  desc "Purpose-built TUI for git stash mastery"
  homepage "https://github.com/indrasvat/nidhi"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_darwin_arm64.tar.gz"
      # sha256 will be filled by goreleaser
    end
    on_intel do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_darwin_amd64.tar.gz"
      # sha256 will be filled by goreleaser
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_linux_arm64.tar.gz"
      # sha256 will be filled by goreleaser
    end
    on_intel do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_linux_amd64.tar.gz"
      # sha256 will be filled by goreleaser
    end
  end

  def install
    bin.install "nidhi"
  end

  def post_install
    ohai "nidhi installed! Run 'nidhi' in any git repo to get started."
    ohai "  Quick start: j/k navigate, Tab preview, Enter detail, ? help"
  end

  test do
    assert_match "nidhi", shell_output("#{bin}/nidhi --version")
  end
end
```

### Step 3: Push and test

```bash
git add Formula/nidhi.rb README.md
git commit -m "feat: add nidhi formula"
git push origin main
```

### Step 4: goreleaser automation

On each nidhi release, goreleaser (via `.goreleaser.yml` brews section)
automatically:
1. Generates a new `Formula/nidhi.rb` with correct URLs and SHA256 checksums
2. Commits it to `indrasvat/homebrew-tap`
3. This requires a `HOMEBREW_TAP_TOKEN` secret in the nidhi repo's GitHub settings

To set up:
```bash
# Create a fine-grained personal access token with:
#   - Repository access: indrasvat/homebrew-tap
#   - Permissions: Contents (read/write)
# Add it as a secret in the nidhi repo:
gh secret set HOMEBREW_TAP_TOKEN --repo indrasvat/nidhi
```

## Testing the Formula

```bash
# Install from the tap.
brew tap indrasvat/tap
brew install nidhi

# Verify.
nidhi --version
nidhi --help

# Test formula from source.
brew install --build-from-source indrasvat/tap/nidhi

# Run Homebrew's formula test.
brew test nidhi
```

## Updating

goreleaser handles updates automatically. Manual update if needed:

```bash
cd homebrew-tap
# Edit Formula/nidhi.rb with new version, URLs, SHA256s.
brew audit --strict Formula/nidhi.rb
git add -A && git commit -m "chore: update nidhi to vX.Y.Z" && git push
```
