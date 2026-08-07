import type { Component } from "solid-js";

interface LandingSectionProps {
  title: string;
  bgColor: string;
  textColor: string;
}

export const LandingSection: Component<LandingSectionProps> = (props) => {
  return (
    <div
      class={`flex-shrink-0 w-screen h-full flex flex-col items-center justify-center ${props.bgColor} ${props.textColor}`}
    >
      <h2 class="text-6xl font-bold">{props.title}</h2>
      <p class="mt-4 text-xl">Content for this section goes here.</p>
    </div>
  );
};
