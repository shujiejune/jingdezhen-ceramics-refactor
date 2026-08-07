import { createFileRoute } from "@tanstack/solid-router";
import { createQuery } from "@tanstack/solid-query";
import { Show, Suspense, onMount } from "solid-js";
import { marked } from "marked";
import type { Component } from "solid-js";

// --- Data Fetching and Types ---

// Define the shape of our article data
interface Article {
  title: string;
  author: string;
  createdAt: string; // ISO 8601 format date string
  updatedAt: string; // ISO 8601 format date string
  content: string; // Markdown content
}

// Mock function to simulate fetching an article from your Go backend
const fetchArticleBySlug = async (slug: string): Promise<Article> => {
  console.log(`Fetching article for slug: ${slug}...`);
  await new Promise((r) => setTimeout(r, 500)); // Simulate network delay

  // In a real app, this would be a fetch call to your API:
  // const response = await fetch(`https://your-api.com/articles/${slug}`);
  // if (!response.ok) throw new Error("Article not found");
  // return response.json();

  // For now, return mock data
  return {
    title: `The Glorious Blue and White Porcelain of the ${slug.charAt(0).toUpperCase() + slug.slice(1)} Dynasty`,
    author: "Dr. Evelyn Reed",
    createdAt: "2025-08-19T10:30:00Z",
    updatedAt: "2025-08-19T15:33:00Z",
    content: `
![Ceramic Vase](https://placehold.co/800x400/d1e5f0/3a6a8a?text=Dynasty+Artwork)

The **Ming Dynasty** (1368–1644) is often hailed as the zenith of Chinese blue and white porcelain production. The imperial kilns at Jingdezhen, under direct court patronage, produced wares of unparalleled quality and artistic sophistication.

### Key Characteristics

- **Imperial Control**: The establishment of the Imperial kilns at Jingdezhen ensured a consistent supply of the finest raw materials and the most skilled artisans.
- **Cobalt Blue**: Potters perfected the use of cobalt oxide imported from Persia, known as "Mohammedan blue," which produced a vibrant, deep blue color that was free from the "heaping and piling" effect of earlier periods.
- **Variety of Forms**: While early Ming wares focused on large, robust forms like chargers and jars, later periods saw the production of delicate bowls, cups, and vases with intricate designs.

### Notable Periods

1.  **Yongle (1403-1424)**: Known for its sweet-white (tianbai) body and fine, elegant painting.
2.  **Xuande (1426-1435)**: Often considered the peak of blue and white production, with a rich, "orange-peel" glaze and bold, powerful designs.
3.  **Chenghua (1465-1487)**: Famous for the development of delicate "doucai" enamels, but also produced exceptional blue and white wares.

The legacy of Ming blue and white porcelain is immense, influencing ceramic traditions across Asia and Europe for centuries to come.
    `,
  };
};

// --- Route Definition with Loader ---

export const Route = createFileRoute("/engage/$slug")({
  // The loader function runs on the server (for SSR) or before the component renders.
  // It fetches the necessary data for this page.
  loader: ({ params: { slug } }) => fetchArticleBySlug(slug),
  component: ArticlePage,
});

// --- Helper Functions ---

// Formats an ISO date string to "YYYY-MM-DD HH:MM"
function formatDateTime(isoString: string) {
  const date = new Date(isoString);
  const pad = (num: number) => num.toString().padStart(2, "0");

  const year = date.getFullYear();
  const month = pad(date.getMonth() + 1);
  const day = pad(date.getDate());
  const hours = pad(date.getHours());
  const minutes = pad(date.getMinutes());

  return `${year}-${month}-${day} ${hours}:${minutes}`;
}

// Calculates estimated read time
function calculateReadTime(text: string): number {
  const wordsPerMinute = 200;
  const wordCount = text.split(/\s+/).length;
  return Math.ceil(wordCount / wordsPerMinute);
}

// --- Main Page Component ---

function ArticlePage() {
  // Use the `useLoaderData` hook to get the data fetched by our loader
  const article = Route.useLoaderData();

  return (
    <div class="bg-white min-h-full">
      <article class="max-w-3xl mx-auto p-8 lg:p-12">
        {/* Article Header */}
        <header class="text-center mb-8">
          <h1 class="text-4xl md:text-5xl font-bold text-gray-800 leading-tight">
            {article().title}
          </h1>
          <div class="mt-4 text-sm text-gray-500 flex flex-wrap justify-center items-center gap-x-4 gap-y-2">
            <span>
              By <strong>{article().author}</strong>
            </span>
            <span class="text-gray-300">|</span>
            <span>Created: {formatDateTime(article().createdAt)}</span>
            <span class="text-gray-300">|</span>
            <span>Updated: {formatDateTime(article().updatedAt)}</span>
            <span class="text-gray-300">|</span>
            <span>{calculateReadTime(article().content)} min read</span>
          </div>
        </header>

        {/* Article Content */}
        <MarkdownRenderer content={article().content} />
      </article>
    </div>
  );
}

// --- Markdown Rendering Component ---
// This component safely renders the Markdown content as HTML.
const MarkdownRenderer: Component<{ content: string }> = (props) => {
  // 1. Component Setup: when calling MarkdownRenderer, container is undefined
  let container: HTMLDivElement | undefined;

  // 4. onMount Hooks: We are guaranteed that container is a real HTML element,
  // it's now safe to access its properties like innerHTML
  onMount(async () => {
    if (container) {
      // The `marked` library converts the markdown string to an HTML string.
      // We then set this HTML as the content of our div.
      // If wrote container.innerHTML = await ..., it would try to run in Step 1,
      // when container is still undefined.
      const htmlContent = await marked.parse(props.content);
      container.innerHTML = htmlContent;
    }
  });

  // 2. JSX Return: this is a description of HTML that needs to be created
  // We add Tailwind's `prose` classes for beautiful default typography styling
  return <div ref={container} class="prose lg:prose-xl max-w-none" />;
  // 3. Mount to DOM: Solid.js takes the description and created the <div> element in the browser's DOM
  // It sees ref={container} and assigns the newly created <div> to the container variable.
};
