class Pgtransfer < Formula
  desc "Transfer PostgreSQL data between databases or CSV files"
  homepage "https://github.com/andymarthin/pgtransfer"
  license "MIT"

  # The version is parsed from the release URL, so it is not declared here.
  # The `Update formula` workflow only needs to rewrite the sha256 values.
  on_macos do
    on_arm do
      url "https://github.com/andymarthin/pgtransfer/releases/download/v1.1.0/pgtransfer-v1.1.0-darwin-arm64.tar.gz"
      sha256 "adb5cbe25a7d2df9acb56146992b31cf7c033627c48a4a9d22bf1ae3136c3812"
    end

    on_intel do
      url "https://github.com/andymarthin/pgtransfer/releases/download/v1.1.0/pgtransfer-v1.1.0-darwin-amd64.tar.gz"
      sha256 "5d2accc5b302c960f45ad986a9ae8d34516e61b1f79f7a7075d4cb64f548683f"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/andymarthin/pgtransfer/releases/download/v1.1.0/pgtransfer-v1.1.0-linux-arm64.tar.gz"
      sha256 "134e10799f04b50ea0515d04bf26cdfd5450ff49f20c53d1461983a5ca59749b"
    end

    on_intel do
      url "https://github.com/andymarthin/pgtransfer/releases/download/v1.1.0/pgtransfer-v1.1.0-linux-amd64.tar.gz"
      sha256 "f614e08eee649e67f5034da3dfaadf200040b1b1fef2f734bd3c133cbc3d028a"
    end
  end

  def install
    bin.install "pgtransfer"
  end

  test do
    assert_match "pgtransfer", shell_output("#{bin}/pgtransfer version")
  end
end
