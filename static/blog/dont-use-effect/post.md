---
title: 'When not to useEffect'
description: 'This is a common antipattern we need to collectively avoid'
pubDate: '2022-12-17'
heroImage: './useffect-post.jpg'
---

React's useEffect hook is a powerful tool for managing side effects in functional components. It allows you to perform tasks that would normally be done in a component's lifecycle methods, such as making network requests or subscribing to events.

However, it is important to understand when not to use useEffect, as it can be overused and is often a footgun in the sense that it can cause problems if used improperly. It is important to carefully consider the consequences of using useEffect and to use it with caution to avoid potential problems.

## Prop changes and the role of useEffect

One common use case for useEffect is updating the component's state based on prop changes. For example, let's say we have a component that displays a list of posts, and also takes an optional `postName` to filter posts by.

```typescript
import { useState, useEffect } from "react";

const PostList = ({ posts, postName }) => {
  const [displayedPosts, setDisplayedPosts] = useState(posts);

  useEffect(() => {
    if (postName) {
      setDisplayedPosts(posts.filter((post) => post.name.includes(postName)));
    } else {
      setDisplayedPosts(posts);
    }
  }, [posts, postName]);

  return (
    <ul>
      {displayedPosts.map((post) => (
        <li key={post.id}>{post.name}</li>
      ))}
    </ul>
  );
};
```

This component uses the useState hook to manage the state of the displayed posts, and the useEffect hook to update the displayed posts based on the list of posts and the post name filter. The `useEffect` hook runs on every change to the posts and postName, this approach works, but has it's pit falls.

## Avoiding unnecessary re-renders

Every time the items prop changes, the component will re-render with the updated `displayedPosts` state. This may not be a problem if the prop changes infrequently, but if it changes frequently (such as in a real-time data application), the component will re-render unnecessarily, which can impact performance.

## Indirection

Additionally, using useEffect to update the component's state based on prop changes adds an extra layer of [indirection](https://en.wikipedia.org/wiki/Indirection) to the component. If a developer goes to the definition of the `displayedItems` they will need to also then find all usages of the `setDisplayedPosts` callback, causing some level of confusion

## Mutability

Another problem with using a `useState` in conjunction with a `useEffect` is that, especially with bigger components, it's not clear when or where a useState is being updated, if there are multiple calls to `setDisplayedPosts` in a file it may add a large amount of cognitive overhead to understand what is happening.

## Alternative approaches

Instead of using useEffect to update the component's state based on prop changes, consider using an inline constant or an inline constant with a useMemo hook for slower operations.

For example, instead of using useEffect and useState in the PostList component, we can simply create an inline const, `displayedPosts`

```typescript
import { useState, useEffect } from "react";

const PostList = ({ posts, postName }) => {
  const displayedPosts = postName
    ? posts.filter((post) => post.name.includes(postName))
    : posts;

  return (
    <ul>
      {displayedPosts.map((post) => (
        <li key={post.id}>{post.name}</li>
      ))}
    </ul>
  );
};
```

This approach avoids the unnecessary re-renders caused by useEffect and keeps the component simpler and more maintainable.

However, sometimes opperations may be expensive. Using an inline constant with `useMemo` allows us to perform the calculation only when the `posts` or `postName` props change, avoiding unnecessary computation whilst still keeping the component simple and maintainable.

```typescript
import { useState, useEffect, useMemo } from "react";

const PostList = ({ posts, postName }) => {
  const displayedPosts = useMemo(
    () =>
      posts.filter((post) => {
        // ... some expensive operation
      }),
    [posts, postName]
  );

  return (
    <ul>
      {displayedPosts.map((post) => (
        <li key={post.id}>{post.name}</li>
      ))}
    </ul>
  );
};
```

ℹ️ Note useMemo is not suitable for async operations such as data fetching, as it blocks the main thread, for these use cases (side effects) useEffect and a useState is a very valid combination, or I'd also reccomend utilizing [@tanstack/react-query](https://tanstack.com/query/v4) for this!

## Conclusion

In conclusion, it is important to use caution when using the React useEffect hook and to carefully consider the consequences of its usage. In some cases, it may be more efficient and maintainable to use inline constants or useMemo to update the component's state based on prop changes, rather than relying on useEffect. It is essential to be aware of the potential for unnecessary re-renders and mutability when using useEffect, and to choose the most appropriate approach for the specific use case.
