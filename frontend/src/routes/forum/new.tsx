import { createFileRoute } from "@tanstack/solid-router";
import { createSignal, For } from "solid-js";
import type { Component } from "solid-js";
import * as Select from "@kobalte/core/select";
import { MarkdownEditor } from "~/components/shared/MarkdownEditor";
import { X, CaretDown } from "~/components/icons/Phosphor";
import "./new.css";

const categories = [
  "Events",
  "Discussion",
  "How To",
  "Showcase",
  "Feedback",
] as const;
type Category = (typeof categories)[number];

export const Route = createFileRoute("/forum/new")({
  component: NewPostPage,
});

function NewPostPage() {
  const [title, setTitle] = createSignal("");
  const [category, setCategory] = createSignal<Category | null>(null);
  const [tags, setTags] = createSignal<string[]>([]);
  const [body, setBody] = createSignal("");

  const handleTagInput = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      const input = e.currentTarget as HTMLInputElement;
      const newTag = input.value.trim();
      if (newTag && !tags().includes(newTag)) {
        setTags((prev) => [...prev, newTag]);
      }
      input.value = "";
    }
  };

  const removeTag = (tagToRemove: string) => {
    setTags((prev) => prev.filter((tag) => tag !== tagToRemove));
  };

  return (
    <div class="bg-white min-h-full">
      <main class="new-post-layout">
        <h1 class="text-2xl font-bold text-gray-800">Start a new discussion</h1>

        <div class="form-section">
          <label for="title" class="form-label">
            Add a title
          </label>
          <input
            id="title"
            type="text"
            class="form-input"
            placeholder="What's on your mind?"
            value={title()}
            onInput={(e) => setTitle(e.currentTarget.value)}
          />
        </div>

        <div class="form-section">
          <label class="form-label">Select the category</label>
          <Select.Root<Category>
            options={[...categories]}
            value={category()}
            onChange={setCategory}
            placeholder="Choose a category..."
            itemComponent={(props) => (
              <Select.Item item={props.item} class="select-item">
                <Select.ItemLabel>{props.item.rawValue}</Select.ItemLabel>
              </Select.Item>
            )}
          >
            <Select.Trigger class="select-trigger">
              <Select.Value<Category>>
                {(state) => state.selectedOption() || "Choose a category..."}
              </Select.Value>
              <Select.Icon>
                <CaretDown />
              </Select.Icon>
            </Select.Trigger>
            <Select.Portal>
              <Select.Content class="select-content">
                <Select.Listbox />
              </Select.Content>
            </Select.Portal>
          </Select.Root>
        </div>

        <div class="form-section">
          <label for="tags" class="form-label">
            Add tags
          </label>
          <div class="tag-input-container">
            <For each={tags()}>
              {(tag) => (
                <span class="tag-capsule">
                  {tag}
                  <button onClick={() => removeTag(tag)} class="tag-remove-btn">
                    <X size={12} />
                  </button>
                </span>
              )}
            </For>
            <input
              id="tags"
              type="text"
              class="tag-input"
              placeholder="Add up to 5 tags..."
              onKeyDown={handleTagInput}
            />
          </div>
        </div>

        <div class="form-section">
          <label class="form-label">Add a body</label>
          <MarkdownEditor onValueChange={setBody} />
        </div>

        <div class="form-actions">
          <button class="submit-post-button">Start discussion</button>
        </div>
      </main>
    </div>
  );
}
