import type { Component } from "solid-js";

interface PageHeaderProps {
  title: string;
  subtitle: string;
}

export const PageHeader: Component<PageHeaderProps> = (props) => {
  return (
    <header class="text-center py-12 px-4">
      <h1 class="text-5xl font-bold text-gray-800 tracking-tight">
        {props.title}
      </h1>
      <p class="mt-4 max-w-2xl mx-auto text-lg text-gray-500">
        {props.subtitle}
      </p>
    </header>
  );
};
