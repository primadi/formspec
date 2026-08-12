#!/usr/bin/env ruby
# frozen_string_literal: true

# Example lib-formspec-ruby app: the business-logic side of an
# impl: {type: sidecar} action. Run inside a pod next to formspec-sidecar:
#
#   FORMA_APP_SOCKET=/tmp/formspec/app.sock \
#   FORMA_SIDECAR_SOCKET=/tmp/formspec/sidecar.sock \
#   ruby examples/app.rb

$LOAD_PATH.unshift(File.join(__dir__, "..", "lib"))
require "lib-formspec"

app = FormSpec::App.new

app.handle("billing.invoice.approve") do |inv, ctx|
  lock_key = "invoice:#{inv.resource_id}"

  unless ctx.lock.acquire(lock_key, 30)
    raise "invoice is being processed by someone else"
  end

  begin
    status = inv.resource["status"]
    raise "only draft invoices can be approved" unless status == "draft"

    FormSpec::ActionResult.new(
      { approved_at: Time.now.utc.iso8601, note: inv.params["note"] || "" },
      new_state: "approved"
    ).with_event("invoice.approved", { id: inv.resource_id }, durable: true)
  ensure
    ctx.lock.release(lock_key)
  end
end

app.run
