import type { Metadata } from "next";
import { IBM_Plex_Mono, Libre_Franklin } from "next/font/google";
import "./globals.css";

const sans = Libre_Franklin({
  variable: "--font-libre",
  subsets: ["latin"],
  weight: ["400", "500", "600"],
});

const mono = IBM_Plex_Mono({
  variable: "--font-ibm-plex-mono",
  subsets: ["latin"],
  weight: ["400", "500"],
});

export const metadata: Metadata = {
  title: "Clearing desk",
  description: "Mock payment lifecycle: authorize, capture, refund, cancel.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${sans.variable} ${mono.variable} h-full`}>
      <body className="min-h-full font-sans antialiased">{children}</body>
    </html>
  );
}
