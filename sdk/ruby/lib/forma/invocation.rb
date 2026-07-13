module Forma
  # One action invocation from the sidecar — the wire form of the engine's
  # ExecuteParams (docs/runtimes/04-forma-sidecar.md §4.2).
  Invocation = Data.define(
    :module,
    :entity,
    :action,
    :resource_id,
    :resource,
    :params,
    :user_id
  ) do
    # @param body [Hash] decoded /invoke request body
    def self.from_request(mod, ent, act, body)
      new(
        module:      mod,
        entity:      ent,
        action:      act,
        resource_id: body["resource_id"].to_s,
        resource:    body["resource"] || {},
        params:      body["params"] || {},
        user_id:     body["user_id"].to_s
      )
    end
  end
end
