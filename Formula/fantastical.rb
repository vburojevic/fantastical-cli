class Fantastical < Formula
  desc "CLI for Fantastical URL handler and AppleScript"
  homepage "https://github.com/vburojevic/fantastical-cli"
  head "https://github.com/vburojevic/fantastical-cli.git", branch: "main"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X main.version=#{version}
    ]
    system "go", "build", "-trimpath", "-ldflags", ldflags.join(" "), "-o", bin/"fantastical", "."
  end

  test do
    assert_match "fantastical", shell_output("#{bin}/fantastical --version")
  end
end
