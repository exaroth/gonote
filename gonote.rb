# Documentation: https://github.com/Homebrew/brew/blob/master/docs/Formula-Cookbook.md
#                http://www.rubydoc.info/github/Homebrew/brew/master/Formula
# PLEASE REMOVE ALL GENERATED COMMENTS BEFORE SUBMITTING YOUR PULL REQUEST!

class Gonote < Formula
  desc "Terminal client for SimpleNote"
  homepage "https://github.com/exaroth/gonote"
  url "https://github.com/exaroth/gonote/releases/download/0.1.0/gonote"
  sha256 "e1e173477eb1c7c49984bea361c56b63f29cdbc387ce707a7d6084813baa437b"

  def install
	bin.install "gonote"
  end

  test do
    system "false"
  end
end
