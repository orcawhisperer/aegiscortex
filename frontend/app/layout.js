import "./globals.css";

export const metadata = {
  title: "AegisCortex",
  description: "Eleven questions. One route. Costs modeled, not spent.",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
