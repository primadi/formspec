// @vitest-environment jsdom
//
// ─── LoginScreen — Chromium password-form guidance ───
//
// Password managers can only pair, fill and save credentials correctly when the
// form tells them what each field is. These assertions pin the contract from
// https://www.chromium.org/developers/design-documents/create-amazing-password-forms/
//
// Two real bugs motivated them:
//   1. The register form advertised `current-password`, so Chrome offered an
//      existing credential on sign-up and saved the wrong thing.
//   2. When the workspace came from the URL the field was dropped from the DOM
//      entirely, leaving the manager with no context for the saved credential.

import { afterEach, describe, expect, it, vi } from "vitest"
import { cleanup, fireEvent, render } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import "@testing-library/jest-dom/vitest"
import type { ComponentProps } from "react"
import { LoginScreen } from "./LoginScreen"

afterEach(cleanup)

function renderLogin(props: Partial<ComponentProps<typeof LoginScreen>> = {}) {
  return render(
    <MemoryRouter initialEntries={["/login"]}>
      <Routes>
        <Route path="*" element={<LoginScreen onLogin={vi.fn()} {...props} />} />
      </Routes>
    </MemoryRouter>,
  )
}

const input = (container: HTMLElement, selector: string) =>
  container.querySelector<HTMLInputElement>(selector)

describe("LoginScreen — autofill contract", () => {
  it("uses the login tokens for username and password", () => {
    const { container } = renderLogin({ mode: "login" })

    expect(input(container, "input[name=username]")).toHaveAttribute(
      "autocomplete",
      "username",
    )
    expect(input(container, "input[name=password]")).toHaveAttribute(
      "autocomplete",
      "current-password",
    )
  })

  it("marks the register password as new-password", () => {
    const { container } = renderLogin({ mode: "register" })

    expect(input(container, "input[name=password]")).toHaveAttribute(
      "autocomplete",
      "new-password",
    )
    expect(input(container, "input[name=display_name]")).toHaveAttribute(
      "autocomplete",
      "name",
    )
    expect(input(container, "input[name=email]")).toHaveAttribute(
      "autocomplete",
      "email",
    )
  })

  it("keeps the workspace as hidden context when it comes from the URL", () => {
    const { container } = renderLogin({ workspace: "cafe", app: "pos" })

    // The typed field is gone (the workspace is already known) but a hidden
    // input still tells the password manager which account set this belongs to.
    expect(input(container, "input#workspace")).toBeNull()
    expect(input(container, "input[type=hidden][name=workspace]")).toHaveValue(
      "cafe",
    )
    expect(input(container, "input[type=hidden][name=app]")).toHaveValue("pos")
  })

  it("gives every credential field a name for autofill heuristics", () => {
    const { container } = renderLogin({ mode: "login" })

    for (const name of ["username", "password", "workspace"]) {
      expect(input(container, `input[name=${name}]`)).not.toBeNull()
    }
  })

  it("uses the email token on the forgot-password form", () => {
    const { container, getByText } = renderLogin({ workspace: "cafe" })

    fireEvent.click(getByText("Forgot password?"))

    expect(input(container, "input[name=email]")).toHaveAttribute(
      "autocomplete",
      "email",
    )
    // No stale login form left behind: the two processes are separate <form>s.
    expect(input(container, "input[name=password]")).toBeNull()
  })
})
