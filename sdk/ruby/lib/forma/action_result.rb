module Forma
  # Structured handler result — the wire form of the engine's ExecuteResult.
  # Handlers may also return plain data (Hash/Array), which becomes +data+.
  class ActionResult
    attr_reader :data, :new_state, :events

    # @param data      [Object] response payload
    # @param new_state [String, nil] new entity state after this action
    def initialize(data = nil, new_state: nil)
      @data      = data
      @new_state = new_state
      @events    = []
    end

    # Add an event emission to the result.
    #
    # @param name    [String] event name
    # @param payload [Hash]   event payload (optional)
    # @param durable [Boolean] whether the event is durable
    # @return [ActionResult] a new instance with the event appended
    def with_event(name, payload = {}, durable: false)
      event = { "name" => name }
      event["payload"] = payload unless payload.nil? || payload.empty?
      event["durable"] = true if durable

      result = dup
      result.instance_variable_set(:@events, @events.dup << event)
      result
    end

    # Serialize to the wire format expected by forma-sidecar.
    # @return [Hash]
    def to_wire
      wire = { "data" => @data }
      wire["new_state"] = @new_state if @new_state
      wire["events"]    = @events    unless @events.empty?
      wire
    end
  end
end
