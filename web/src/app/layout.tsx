import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Scheme Setu — find the government schemes you qualify for",
  description:
    "Answer six questions and see every Indian government loan and subsidy you qualify for, with the real application link, the documents you need, proof it is still open, and a warning about fake portals.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
