module Forma
  # One ctx primitive handle (db/cache/lock/...), optionally bound to a named
  # datastore via +#named+. Operations map 1:1 to POST /ctx/{prim}/{op}.
  class CtxPrimitive
    # @param client     [SidecarClient]
    # @param type       [String]  primitive type ("db", "cache", "lock", ...)
    # @param bound_name [String]  named datastore (empty = default)
    def initialize(client, type, bound_name = "")
      @client     = client
      @type       = type
      @bound_name = bound_name
    end

    # Bind to a named datastore instead of the default one.
    # @param name [String]
    # @return [CtxPrimitive]
    def named(name)
      self.class.new(@client, @type, name)
    end

    # ---- db ----

    # Execute a query on the datastore.
    #
    # @param sql  [String]  SQL query
    # @param args [Array]   positional arguments (optional)
    # @return [Array<Hash>] result rows
    def query(sql, *args)
      body = { "sql" => sql }
      body["args"] = args unless args.empty?
      resp = call("query", body)
      resp["data"] || []
    end

    # ---- cache / kvstore ----

    # Get a value by key.
    # @return [Object, nil]
    def get(key)
      call("get", { "key" => key })["data"]
    end

    # Set a value by key.
    #
    # @param key        [String]
    # @param value      [Object]
    # @param ttl_seconds [Integer] TTL in seconds (0 = no expiry)
    def set(key, value, ttl_seconds = 0)
      body = { "key" => key, "value" => value }
      body["ttl_seconds"] = ttl_seconds if ttl_seconds > 0
      call("set", body)
    end

    # Delete a key.
    def delete(key)
      call("delete", { "key" => key })
    end

    # ---- lock ----

    # Acquire a distributed lock.
    #
    # @param key         [String]
    # @param ttl_seconds [Integer] lock TTL (default 30)
    # @return [Boolean] true if the lock was acquired
    def acquire(key, ttl_seconds = 30)
      resp = call("acquire", { "key" => key, "ttl_seconds" => ttl_seconds })
      resp["ok"] == true
    end

    # Release a distributed lock.
    def release(key)
      call("release", { "key" => key })
    end

    private

    def call(op, body)
      body["named"] = @bound_name unless @bound_name.empty?
      @client.post("/ctx/#{@type}/#{op}", body)
    end
  end
end
