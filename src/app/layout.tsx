import "./globals.css"; // Подключение глобальных стилей

export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <title>Игра "Начало"</title>
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      </head>
      <body>
        <header className="header">
          <h1>Игра "Начало"</h1>
        </header>
        <main>{children}</main>
        <footer className="footer">
          <p>Все права защищены © 2024</p>
        </footer>
      </body>
    </html>
  );
}
