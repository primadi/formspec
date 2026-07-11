Gem::Specification.new do |s|
  s.name        = "lib-forma"
  s.version     = "0.1.0"
  s.summary     = "Thin client SDK bridging a Ruby app to forma-sidecar"
  s.description = "/invoke listener + ctx.* proxy client. See docs/runtimes/04-forma-sidecar.md"
  s.authors     = ["Forma"]
  s.license     = "FSL-1.1-ALv2"
  s.homepage    = "https://github.com/primadi/forma"

  s.files       = Dir["lib/**/*.rb"]
  s.require_paths = ["lib"]

  s.required_ruby_version = ">= 3.0"

  # No runtime dependencies — stdlib only
end
