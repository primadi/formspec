import { Nav } from "./components/Nav"
import { Hero } from "./components/Hero"
import { Primitives } from "./components/Primitives"
import { Architecture } from "./components/Architecture"
import { ImplTypes } from "./components/ImplTypes"
import { Derived } from "./components/Derived"
import { Marketplace } from "./components/Marketplace"
import { Quickstart } from "./components/Quickstart"
import { CTA } from "./components/CTA"
import { Footer } from "./components/Footer"

export default function App() {
  return (
    <div className="min-h-screen bg-surface-950 text-zinc-200">
      <Nav />
      <main>
        <Hero />
        <Primitives />
        <Architecture />
        <ImplTypes />
        <Derived />
        <Marketplace />
        <Quickstart />
        <CTA />
      </main>
      <Footer />
    </div>
  )
}
