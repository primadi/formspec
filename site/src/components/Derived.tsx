import { LayoutDashboard, ListOrdered, FormInput, Menu } from "lucide-react"

const DERIVED = [
  {
    icon: ListOrdered,
    title: "Table",
    desc: "List/browse per entity — kolom, filter, sort, pagination otomatis.",
  },
  {
    icon: FormInput,
    title: "Forms",
    desc: "Form create & edit diturunkan dari field definition, lengkap dengan validasi.",
  },
  {
    icon: LayoutDashboard,
    title: "Detail Page",
    desc: "Halaman detail dengan relasi dan riwayat state.",
  },
  {
    icon: Menu,
    title: "Menu",
    desc: "Menu admin panel tersusun otomatis dari module dan views.",
  },
]

export function Derived() {
  return (
    <section className="border-y border-white/5 bg-surface-900/40 py-24">
      <div className="mx-auto grid max-w-6xl items-center gap-12 px-5 lg:grid-cols-2">
        <div>
          <p className="text-sm font-semibold uppercase tracking-widest text-cyan-400">
            Derived by Default
          </p>
          <h2 className="mt-3 text-3xl font-bold text-white sm:text-4xl">
            Admin panel muncul sendiri dari Entity
          </h2>
          <p className="mt-4 text-zinc-400">
            Setiap Entity otomatis menghasilkan Table, Forms (create/edit),
            detail Page, dan Menu entries. ~80% UI yang patterned datang gratis;
            ~20% sisanya kamu kontrol lewat manifest dan custom component.
          </p>
          <p className="mt-4 text-sm text-zinc-500">
            Rendering terjadi saat runtime — satu interpreter di-deploy sekali,
            membaca spec untuk App/Page apa pun.
          </p>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          {DERIVED.map((d) => (
            <div
              key={d.title}
              className="rounded-2xl border border-white/5 bg-surface-900 p-5 transition-colors hover:border-cyan-400/30"
            >
              <d.icon className="mb-3 size-6 text-cyan-400" />
              <h3 className="font-semibold text-white">{d.title}</h3>
              <p className="mt-1.5 text-sm text-zinc-400">{d.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
