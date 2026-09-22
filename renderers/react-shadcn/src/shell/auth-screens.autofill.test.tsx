// @vitest-environment jsdom
//
// ─── Auth screens — password manager contract ───
//
// The login/register path is covered in LoginScreen.test.tsx. These are the
// remaining screens that collect a password: first-run setup, the emailed
// reset link, and the authenticated change-password page. All three know their
// workspace from the URL and never show it as a field — so without a hidden
// input the password manager has no context for the credential it saves.
// Reference:
// https://www.chromium.org/developers/design-documents/create-amazing-password-forms/

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, render } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import "@testing-library/jest-dom/vitest"
import { SetupScreen } from "./SetupScreen"
import { ResetPasswordScreen } from "./ResetPasswordScreen"
import { ChangePasswordPage } from "./ChangePasswordPage"

beforeEach(() => {
  // SetupScreen probes the workspace on mount to detect an already-completed
  // setup; keep it pending-but-successful so the form stays rendered.
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ data: { setup_required: true } }),
      }),
    ),
  )
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function renderAt(path: string, element: React.ReactNode) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/:workspace/*" element={element} />
      </Routes>
    </MemoryRouter>,
  ).container
}

const field = (container: HTMLElement, name: string) =>
  container.querySelector<HTMLInputElement>(`input[name="${name}"]`)

const hiddenWorkspace = (container: HTMLElement) =>
  container.querySelector<HTMLInputElement>(
    'input[type="hidden"][name="workspace"]',
  )

describe("SetupScreen", () => {
  it("names the credential fields and keeps the workspace as hidden context", () => {
    const container = renderAt("/cafe/_admin/setup", <SetupScreen />)

    expect(hiddenWorkspace(container)).toHaveValue("cafe")
    expect(field(container, "username")).toHaveAttribute(
      "autocomplete",
      "username",
    )
    expect(field(container, "password")).toHaveAttribute(
      "autocomplete",
      "new-password",
    )
    expect(field(container, "confirm-password")).toHaveAttribute(
      "autocomplete",
      "new-password",
    )
  })
})

describe("ResetPasswordScreen", () => {
  it("carries workspace + single-use token as hidden context", () => {
    const container = renderAt(
      "/cafe/reset-password?reset_token=tok-123",
      <ResetPasswordScreen />,
    )

    expect(hiddenWorkspace(container)).toHaveValue("cafe")
    expect(container.querySelector('input[name="token"]')).toHaveValue("tok-123")
    expect(field(container, "password")).toHaveAttribute(
      "autocomplete",
      "new-password",
    )
  })
})

describe("ChangePasswordPage", () => {
  it("distinguishes the existing credential from the new one", () => {
    const container = renderAt("/cafe/_admin/change-password", (
      <ChangePasswordPage />
    ))

    expect(hiddenWorkspace(container)).toHaveValue("cafe")
    expect(field(container, "current-password")).toHaveAttribute(
      "autocomplete",
      "current-password",
    )
    expect(field(container, "new-password")).toHaveAttribute(
      "autocomplete",
      "new-password",
    )
  })
})
