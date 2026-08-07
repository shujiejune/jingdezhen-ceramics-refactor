import { createFileRoute } from "@tanstack/solid-router";
import { LandingSection } from "~/components/landing/LandingSection";

export const Route = createFileRoute("/")({
  component: Index,
});

function Index() {
  const sections = [
    {
      title: "CeramicStory",
      bgColor: "bg-blue-100",
      textColor: "text-white-800",
    },
    { title: "Gallery", bgColor: "bg-green-100", textColor: "text-black-800" },
    { title: "Engage", bgColor: "bg-yellow-100", textColor: "text-black-800" },
    { title: "Courses", bgColor: "bg-green-100", textColor: "text-black-800" },
    { title: "Forum", bgColor: "bg-red-100", textColor: "text-red-800" },
    { title: "Portfolio", bgColor: "bg-blue-100", textColor: "text-white-800" },
    { title: "Contact", bgColor: "bg-yellow-100", textColor: "text-black-800" },
  ];

  return (
    // The main container that hides overflow
    <div class="w-full h-full overflow-hidden">
      {/* The track that contains the scrolling items.
          We add the sections twice for a seamless loop. */}
      <div class="flex h-full animate-infinite-scroll">
        {sections.map((sec) => (
          <LandingSection {...sec} />
        ))}
        {sections.map((sec) => (
          <LandingSection {...sec} />
        ))}
      </div>
    </div>
  )
}
