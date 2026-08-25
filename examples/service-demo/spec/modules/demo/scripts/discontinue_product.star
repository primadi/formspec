# No-op handler untuk transisi state machine `discontinue`.
# Transisi ini di-intercept workflow approval (7.4) — eksekusi transisi
# sebenarnya dilakukan oleh framework setelah approval selesai
# (executeWorkflowTransition), jadi script ini tidak melakukan apa-apa.
def execute(resource, params, ctx):
    return ok({"transition": "discontinue"})