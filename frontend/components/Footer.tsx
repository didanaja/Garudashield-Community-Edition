export default function Footer() {
  return (
    <footer className="border-t border-navy-100 bg-white/60">
      <div className="container-shell flex flex-col items-center justify-between gap-4 py-8 text-sm text-navy-500 sm:flex-row">
        <p>
          <span className="font-semibold text-navy-800">GarudaShield</span> Demo Edition — educational demonstration only.
        </p>
        <p className="text-xs">
          Experimental Protection Engine. Not a production WAF. No fake statistics.
        </p>
      </div>
    </footer>
  );
}
