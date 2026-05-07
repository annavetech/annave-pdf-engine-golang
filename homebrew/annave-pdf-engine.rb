# ANNAVE PDF Engine — Homebrew Formula
#
# Personal tap: github.com/annavetech/homebrew-annave
#
# Install:
#   brew tap annavetech/annave
#   brew install annave-pdf-engine
#
# To update this formula after a new release:
#   1. Run ./scripts/release.sh <version> to build and checksum the artifacts
#   2. Upload the artifacts to the GitHub release
#   3. Replace the sha256 values below with the darwin_arm64 and darwin_amd64
#      values from dist/v<version>/checksums.txt (CLI binary, not server binary)
#   4. Update the version string and URLs
#   5. Commit and push to the homebrew-annave tap repository

class AnnaveCliEngine < Formula
  desc     "Convert documents to PDF from the command line — ANNAVE PDF Engine"
  homepage "https://www.annave.tech"
  version  "1.0.0"
  license  "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/annavetech/annave-pdf-engine-golang/releases/download/v#{version}/annave-pdf-engine_#{version}_darwin_arm64"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT_darwin_arm64"
    end

    on_intel do
      url "https://github.com/annavetech/annave-pdf-engine-golang/releases/download/v#{version}/annave-pdf-engine_#{version}_darwin_amd64"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT_darwin_amd64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/annavetech/annave-pdf-engine-golang/releases/download/v#{version}/annave-pdf-engine_#{version}_linux_arm64"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT_linux_arm64"
    end

    on_intel do
      url "https://github.com/annavetech/annave-pdf-engine-golang/releases/download/v#{version}/annave-pdf-engine_#{version}_linux_amd64"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT_linux_amd64"
    end
  end

  def install
    bin.install Dir["annave-pdf-engine_*"].first => "annave"
  end

  test do
    output = shell_output("#{bin}/annave version")
    assert_match "1.0.0", output

    # Convert a minimal Markdown document to PDF and verify the output is a PDF.
    (testpath/"test.md").write("# Test\n\nHello from Homebrew.\n")
    system bin/"annave", "pdf", "convert", "test.md", "-o", "test.pdf"
    assert_predicate testpath/"test.pdf", :exist?
    # PDF magic bytes: %PDF
    assert_equal "%PDF", (testpath/"test.pdf").read(4)
  end
end
