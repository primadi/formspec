module FormSpec
  # The ctx.* surface handed to handlers. Every method call is an HTTP call
  # back to formspec-sidecar (§4.3) — the same primitive contract Starlark
  # scripts use.
  class Ctx
    def initialize(client)
      @client = client
    end

    def db      = CtxPrimitive.new(@client, "db")
    def cache   = CtxPrimitive.new(@client, "cache")
    def lock    = CtxPrimitive.new(@client, "lock")
    def queue   = CtxPrimitive.new(@client, "queue")
    def pubsub  = CtxPrimitive.new(@client, "pubsub")
    def storage = CtxPrimitive.new(@client, "storage")
    def kvstore = CtxPrimitive.new(@client, "kvstore")

    # Entity primitive — access entity records via named('module/entity').
    def entity = CtxPrimitive.new(@client, "entity")
  end
end
